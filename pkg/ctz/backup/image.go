package backup

import (
	"ceph-to-zfs/pkg/ctz/cephsupport"
	"ceph-to-zfs/pkg/ctz/config"
	"ceph-to-zfs/pkg/ctz/logging"
	"ceph-to-zfs/pkg/ctz/status"
	"ceph-to-zfs/pkg/ctz/task"
	"ceph-to-zfs/pkg/ctz/util"
	"ceph-to-zfs/pkg/ctz/zfssupport"
	"errors"
	"fmt"
	"github.com/ceph/go-ceph/rados"
	"github.com/ceph/go-ceph/rbd"
	"golang.org/x/sys/unix"
	"os"
	"runtime"
	"slices"
	"time"
	"unsafe"
)

// ImageBackupTask represents the backup process for a single image
type ImageBackupTask struct {
	imageName  string
	cephConfig *config.CephClusterConfig
	poolName   string
	ioctx      *rados.IOContext
	zfsContext *zfssupport.ZfsContext
	log        *logging.JobStatusLogger
	mt         *task.ManagedTask
	finalData  *finalData
}

type finalData struct {
	zfsSnapshotName string
	bytesWritten    uint64
	bytesTrimmed    uint64
}

func NewImageBackupTask(imageName string, cephConfig *config.CephClusterConfig, poolname string, zfsContext *zfssupport.ZfsContext, parentLog *logging.JobStatusLogger) *ImageBackupTask {
	log := parentLog.MakeOrReplaceChild(logging.LoggerKey(imageName), true)
	out := &ImageBackupTask{
		imageName: imageName,
		//ioctx:      ioctx,
		cephConfig: cephConfig,
		poolName:   poolname,
		zfsContext: zfsContext,
		log:        log,
	}
	out.mt = task.NewManagedTask(log, out.reset, out.run)
	return out
}

func (t *ImageBackupTask) StatusLog() *logging.JobStatusLogger {
	return t.log
}

// No children
// TODO: maybe represent individual parts of the process as children?
func (t *ImageBackupTask) Children() []task.Task {
	return nil
}

func (t *ImageBackupTask) Label() string {
	return t.imageName
}

func (t *ImageBackupTask) Id() string {
	// TODO: I can't find a concrete source for what characters are allowed in an RBD image name
	return t.imageName
}

func (t *ImageBackupTask) Run() error {
	// Format the success message with the final results
	return t.mt.Run(func() string {
		fd := t.finalData
		if fd == nil {
			return "FAIL: task did not report data"
		} else {
			return fmt.Sprintf("Wrote %v bytes (trimmed %v) and created snapshot '%v'", fd.bytesWritten, fd.bytesTrimmed, fd.zfsSnapshotName)
		}
	})
}

func (t *ImageBackupTask) reset() error {
	t.finalData = nil
	return nil
}

func (t *ImageBackupTask) run() error {
	var bytesWritten uint64
	var bytesTrimmed uint64

	// Snapshot name convention: ctz-YYYY-MM-dd-HH:mm:ss
	// TODO make this configurable
	snapName := "ctz-" + time.Now().Format("2006-01-02-15:04:05")

	t.log.SetStatus(status.SimpleStatus(status.Preparing))
	t.log.SetExtraData("snapName", snapName)

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	t.log.Log("Getting ceph image")
	conn, err := cephsupport.Connect(t.cephConfig)
	if err != nil {
		return util.Wrap("failed to connect to ceph cluster", err)
	}
	defer func() { go conn.Shutdown() }()
	t.log.SetStatus(status.MakeStatus(status.Preparing, "Opening IOContext"))
	context, err := conn.OpenIOContext(t.poolName)
	if err != nil {
		return util.Wrap("error opening IOContext", err)
	}
	img, err := rbd.OpenImage(context, t.imageName, "")
	if err != nil {
		return util.Wrap("error opening image", err)
	}
	defer img.Close()
	cephImage := cephsupport.NewCephImageView(img)

	t.log.SetStatus(status.MakeStatus(status.Preparing, fmt.Sprintf("Creating RBD snapshot %v", snapName)))

	// Get image size to ensure that the receiver is large enough
	size, err := cephImage.Size()
	if err != nil {
		return util.Wrap("error getting ceph image size", err)
	}
	t.log.Log("Ceph image size: %v", size)
	// Snapshot the ceph pool
	err = cephImage.SnapAndActivate(snapName)
	if err != nil {
		return util.Wrap("error preparing ceph image", err)
	}
	//// Also check block size
	// XXX this doesn't work - object size != block size
	//blockSize, err := cephImage.BlockSize()
	//if err != nil {
	//	return err
	//}
	// Prep ZFS side
	t.log.SetStatus(status.MakeStatus(status.Preparing, "Preparing ZFS"))
	zplog := t.log.MakeOrReplaceChild("Find/Create Dataset", true)

	// TODO: this isn't very much a "prep" step
	zv, err := t.zfsContext.PrepareChild(t.Label(), size, zplog)
	if err != nil {
		wrapped := util.Wrap("error preparing zfs dataset", err)
		zplog.SetStatusByError(wrapped)
		return wrapped
	}
	// Find the most recent common snapshot between the two, using the name as the key
	zvolSnaps, err := zv.Snapshots()
	if err != nil {
		return util.Wrap("error getting ZFS snapshots", err)
	}
	cephSnaps, err := cephImage.SnapNames()
	if err != nil {
		return util.Wrap("error getting ceph snaps", err)
	}
	// Reverses in place
	slices.Reverse(cephSnaps)
	// Find most recent snapshot that exists on both ends
	var mostRecentCommon *zfssupport.ZvolSnapshot
	for _, cephSnap := range cephSnaps {
		matching, found := util.FindFirst(zvolSnaps, func(snapshot *zfssupport.ZvolSnapshot) bool {
			return snapshot.Name == cephSnap
		})
		if found {
			mostRecentCommon = *matching
			break
		}
	}
	var mostRecentName string
	if mostRecentCommon == nil {
		t.log.Log("No existing ZFS snapshot")
		mostRecentName = ""
	} else {
		// Force-revert the ZFS side to the most recent common snapshot
		mostRecentName = mostRecentCommon.Name
		t.log.Log("Most recent common snapshot: %v", mostRecentCommon.Name)
		t.log.SetStatus(status.MakeStatus(status.Preparing, fmt.Sprintf("Reverting ZFS to %v", mostRecentName)))
		err = zv.RevertTo(mostRecentCommon)
		if err != nil {
			return util.WrapFmt(err, "error reverting ZFS to %v@%v", t.imageName, mostRecentName)
		}
	}
	var mostRecentNameFmt string
	if mostRecentName == "" {
		mostRecentNameFmt = "(base)"
	} else {
		mostRecentNameFmt = mostRecentName
	}
	t.log.Log("Plan: %v -> %v", mostRecentNameFmt, snapName)

	node := zv.DevNode()
	t.log.SetStatus(status.MakeStatus(status.Preparing, "Opening zvol device node"))
	var file *os.File
	for tries := 5; tries > 0; {
		tries--
		var fileErr error
		file, fileErr = os.OpenFile(node, os.O_WRONLY, 600)
		if fileErr != nil {
			if tries <= 0 {
				return util.WrapFmt(fileErr, "Failed to open Zvol device %v", node)
			} else {
				t.log.Log("Retrying to open zvol device node (error: %v)", fileErr)
				time.Sleep(5 * time.Second)
			}
		}
	}

	defer func() {
		if file != nil {
			file.Close()
		}
	}()

	t.log.SetStatus(status.MakeStatus(status.InProgress, "Copying data"))

	// TODO: allow buffering between the reads and writes
	var diffErr error
	err = cephImage.DiffIter(mostRecentName, func(offset uint64, length uint64, exists int, _ interface{}) int {
		if exists > 0 {
			bytes, errInner := cephImage.Read(offset, length)
			if errInner != nil {
				diffErr = errInner
				return 1
			}
			_, errInner = file.WriteAt(bytes, int64(offset))
			if errInner != nil {
				diffErr = errInner
				return 1
			}
			bytesWritten += length
			t.log.SetExtraData("bytesWritten", bytesWritten)
			return 0
		} else {

			rangeBytes := []byte{
				byte(offset), byte(offset >> 8), byte(offset >> 16), byte(offset >> 24),
				byte(offset >> 32), byte(offset >> 40), byte(offset >> 48), byte(offset >> 56),
				byte(length), byte(length >> 8), byte(length >> 16), byte(length >> 24),
				byte(length >> 32), byte(length >> 40), byte(length >> 48), byte(length >> 56),
			}

			_, _, errno := unix.Syscall(
				unix.SYS_IOCTL,
				file.Fd(),
				uintptr(unix.BLKDISCARD),
				uintptr(unsafe.Pointer(&rangeBytes[0])),
			)
			if errno != 0 {
				diffErr = util.Wrap("error copying data", errors.New("Syscall error: "+errno.Error()))
				return 1
			}
			t.log.SetExtraData("bytesTrimmed", bytesTrimmed)
			bytesTrimmed += length
			return 0
		}
	})
	t.log.SetExtraData("bytesWritten", bytesWritten)
	t.log.SetExtraData("bytesTrimmed", bytesTrimmed)

	if err != nil {
		return util.Wrap("error copying data", err)
	}
	if diffErr != nil {
		return util.Wrap("error copying data", diffErr)
	}

	t.log.SetStatus(status.MakeStatus(status.Finishing, "Flushing"))
	err = file.Close()
	if err != nil {
		return err
	} else {
		file = nil
	}
	t.log.SetStatus(status.MakeStatus(status.Finishing, "Snapshotting"))

	_, err = zv.NewSnapshot(snapName)
	if err != nil {
		return util.Wrap("error creating snapshot", err)
	}
	t.finalData = &finalData{
		zfsSnapshotName: snapName,
		bytesWritten:    bytesWritten,
		bytesTrimmed:    bytesTrimmed,
	}
	return nil
}

var _ task.Task = &ImageBackupTask{}
