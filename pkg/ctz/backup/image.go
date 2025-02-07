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
	"sync"
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
	mut        *sync.Mutex
}

func NewImageBackupTask(imageName string, cephConfig *config.CephClusterConfig, poolname string, zfsContext *zfssupport.ZfsContext, parentLog *logging.JobStatusLogger) *ImageBackupTask {
	return &ImageBackupTask{
		imageName: imageName,
		//ioctx:      ioctx,
		cephConfig: cephConfig,
		poolName:   poolname,
		zfsContext: zfsContext,
		log:        parentLog.MakeOrReplaceChild(logging.LoggerKey(imageName), true),
		mut:        &sync.Mutex{},
	}
}

func (t *ImageBackupTask) StatusLog() *logging.JobStatusLogger {
	return t.log
}

// No children
// TODO: maybe represent individual parts of the process as children?
func (t *ImageBackupTask) Children() []task.Task {
	return nil
}

func (t *ImageBackupTask) Name() string {
	return t.imageName
}

func (t *ImageBackupTask) Run() (err error) {
	var bytesWritten uint64
	var bytesTrimmed uint64
	// lock
	locked := t.mut.TryLock()
	if !locked {
		return errors.New("ImageBackupTask already in progress")
	}
	defer t.mut.Unlock()
	snapName := "ctz-" + time.Now().Format("2006-01-02-15:04:05")

	t.log.SetStatus(status.SimpleStatus(status.Preparing))

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	t.log.Log("Getting ceph image")
	conn, err := cephsupport.Connect(t.cephConfig)
	if err != nil {
		return err
	}
	defer func() { go conn.Shutdown() }()
	t.log.SetStatus(status.MakeStatus(status.Preparing, "Enumerating Images"))
	context, err := conn.OpenIOContext(t.poolName)
	if err != nil {
		return err
	}
	img, err := rbd.OpenImage(context, t.imageName, "")
	if err != nil {
		return err
	}
	defer img.Close()
	cephImage := cephsupport.NewCephImageView(img)

	// Snapshot name convention: ctz-YYYY-MM-dd-HH:mm:ss
	// TODO make this configurable
	t.log.Log("Creating snapshot %v", snapName)

	defer func() {
		if err != nil {
			t.log.SetStatusByError(err)
		} else {
			_ = t.log.SetFinished(fmt.Sprintf("Wrote %v bytes (trimmed %v) and created snapshot '%v'", bytesWritten, bytesTrimmed, snapName))
		}
	}()

	// Get image size to ensure that the receiver is large enough
	size, err := cephImage.Size()
	if err != nil {
		return err
	}
	t.log.Log("Ceph image size: %v", size)
	// Snapshot the ceph pool
	err = cephImage.SnapAndActivate(snapName)
	if err != nil {
		return err
	}
	// Also check block size
	blockSize, err := cephImage.BlockSize()
	if err != nil {
		return err
	}
	// Prep ZFS side
	t.log.Log("Preparing ZFS")
	zplog := t.log.MakeOrReplaceChild("Find/Create Dataset", true)
	// TODO: this isn't very much a "prep" step
	zv, err := t.zfsContext.PrepareChild(t.Name(), size, blockSize, zplog)
	if err != nil {
		zplog.SetStatusByError(err)
		return err
	}
	// Find the most recent common snapshot between the two, using the name as the key
	zvolSnaps, err := zv.Snapshots()
	if err != nil {
		return err
	}
	cephSnaps, err := cephImage.SnapNames()
	if err != nil {
		return err
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
			return err
		}
	}
	var mostRecentNameFmt string
	if mostRecentName == "" {
		mostRecentNameFmt = "(base)"
	} else {
		mostRecentNameFmt = mostRecentName
	}
	t.log.Log("Plan: %v -> %v", mostRecentNameFmt, snapName)

	node, err := zv.DevNode()
	if err != nil {
		return err
	}
	t.log.SetStatus(status.MakeStatus(status.Preparing, "Opening zvol device node"))
	var file *os.File
	for tries := 5; tries > 0; {
		tries--
		var fileErr error
		file, fileErr = os.OpenFile(node, os.O_WRONLY, 600)
		if fileErr != nil {
			if tries <= 0 {
				return err
			} else {
				t.log.Log("Retrying to open zvol device node (error: %v)", fileErr)
				time.Sleep(5 * time.Second)
			}
		}

	}
	//goland:noinspection GoUnhandledErrorResult
	defer func() {
		if file != nil {
			file.Close()
		}
	}()

	t.log.SetStatus(status.MakeStatus(status.InProgress, "Copying data"))

	err = cephImage.DiffIter(mostRecentName, func(offset uint64, length uint64, exists int, _ interface{}) int {
		if exists > 0 {
			bytes, errInner := cephImage.Read(offset, length)
			if err != nil {
				err = errInner
				return 1
			}
			_, errInner = file.WriteAt(bytes, int64(offset))
			if errInner != nil {
				err = errInner
				return 1
			}
			bytesWritten += length
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
				err = errors.New("Syscall error: " + errno.Error())
				return 1
			}
			bytesTrimmed += length
			return 0
		}
	})
	if err != nil {
		return err
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
		return err
	}
	return nil
}

var _ task.Task = &ImageBackupTask{}
