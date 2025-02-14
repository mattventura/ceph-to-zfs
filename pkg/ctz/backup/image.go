package backup

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ceph/go-ceph/rados"
	"github.com/ceph/go-ceph/rbd"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/cephsupport"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/config"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/logging"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/models"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/pruning"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/status"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/task"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/util"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/zfssupport"
	"golang.org/x/sys/unix"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

// ImageBackupTask represents the backup process for a single image (one RBD image to one ZVOL)
type ImageBackupTask struct {
	imageName  string
	cephConfig *config.CephClusterConfig
	srcPruner  pruning.Pruner[*models.CephSnapshot]
	rcvPruner  pruning.Pruner[*zfssupport.ZvolSnapshot]
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

func NewImageBackupTask(
	imageName string,
	cephConfig *config.CephClusterConfig,
	poolname string,
	zfsContext *zfssupport.ZfsContext,
	parentLog *logging.JobStatusLogger,
	jobConfig *config.RbdPoolJobProcessedConfig,
) *ImageBackupTask {
	log := parentLog.MakeOrReplaceChild(logging.LoggerKey(imageName), true)
	out := &ImageBackupTask{
		imageName: imageName,
		//ioctx:      ioctx,
		cephConfig: cephConfig,
		poolName:   poolname,
		zfsContext: zfsContext,
		log:        log,
		srcPruner:  jobConfig.SrcPruning,
		rcvPruner:  jobConfig.RcvPruning,
	}
	out.mt = task.NewManagedTask(log, out.reset, out.run)
	return out
}

func (t *ImageBackupTask) StatusLog() *logging.JobStatusLogger {
	return t.log
}

// No children
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
	zplog := t.log.MakeOrReplaceChild("zfsprep", true)

	// TODO: this isn't very much a "prep" step
	zv, err := t.zfsContext.PrepareChild(t.Label(), size, zplog)
	if err != nil {
		wrapped := util.Wrap("error preparing zfs dataset", err)
		zplog.SetStatusByError(wrapped)
		return wrapped
	}
	// Find the most recent models snapshot between the two, using the name as the key
	zvolSnaps, err := zv.Snapshots()
	if err != nil {
		return util.Wrap("error getting ZFS snapshots", err)
	}
	cephSnapNames, err := cephImage.SnapNames()
	if err != nil {
		return util.Wrap("error getting ceph snaps", err)
	}
	// Reverses in place
	slices.Reverse(cephSnapNames)
	// Find most recent snapshot that exists on both ends
	var mostRecentCommon *zfssupport.ZvolSnapshot
	for _, cephSnap := range cephSnapNames {
		matching, found := util.FindFirst(zvolSnaps, func(snapshot *zfssupport.ZvolSnapshot) bool {
			return snapshot.Name() == cephSnap
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
		// Force-revert the ZFS side to the most recent models snapshot
		mostRecentName = mostRecentCommon.Name()
		t.log.Log("Most recent models snapshot: %v", mostRecentName)
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
	t.log.SetStatus(status.MakeStatus(status.Finishing, "Planning snapshot pruning"))
	cephSnaps, err := cephImage.Snapshots()
	if err != nil {
		return err
	}
	srcDestroy := t.srcPruner.Destroy(cephSnaps)
	srcSnaps := len(cephSnaps)
	srcToDestroy := len(srcDestroy)
	srcToKeep := srcSnaps - srcToDestroy
	t.log.SetExtraData("srcSnaps", srcSnaps)
	t.log.SetExtraData("srcSnapsToDestroy", srcToDestroy)
	t.log.SetExtraData("srcSnapsToKeep", srcToKeep)

	// Refresh the list so that it includes our new snapshots
	zvolSnaps, err = zv.Snapshots()
	if err != nil {
		return err
	}
	rcvDestroy := t.rcvPruner.Destroy(zvolSnaps)
	rcvSnaps := len(rcvDestroy)
	rcvToDestroy := len(rcvDestroy)
	rcvToKeep := rcvSnaps - rcvToDestroy
	t.log.SetExtraData("rcvSnaps", rcvSnaps)
	t.log.SetExtraData("rcvSnapsToDestroy", rcvToDestroy)
	t.log.SetExtraData("rcvSnapsToKeep", rcvToKeep)

	snapReport := t.makeSnapshotReport(cephSnaps, srcDestroy, zvolSnaps, rcvDestroy)
	t.log.SetDetailData("snapshotReport", snapReport)
	for _, snapshot := range snapReport.Snapshots {
		t.log.Log(snapshot.String())
	}

	t.log.SetStatus(status.MakeStatus(status.Finishing, fmt.Sprintf("Pruning %v ceph snapshots", srcToDestroy)))
	pruneErrors := []error{}
	cephPruned := 0
	for _, snapshot := range srcDestroy {
		t.log.Log("Pruning ceph snapshot %v", snapshot.Name())
		err := cephImage.DeleteSnapshot(snapshot)
		if err != nil {
			pruneErrors = append(pruneErrors, err)
		} else {
			cephPruned++
		}
	}
	t.log.SetStatus(status.MakeStatus(status.Finishing, fmt.Sprintf("Pruned %v ceph snapshots", cephPruned)))

	t.log.SetStatus(status.MakeStatus(status.Finishing, fmt.Sprintf("Pruning %v ZFS snapshots", rcvToDestroy)))
	zfsPruned := 0
	for _, snapshot := range rcvDestroy {
		t.log.Log("Pruning ZFS snapshot %v", snapshot.Name())
		err := zv.DeleteSnapshot(snapshot)
		if err != nil {
			pruneErrors = append(pruneErrors, err)
		} else {
			zfsPruned++
		}
	}
	t.log.SetStatus(status.MakeStatus(status.Finishing, fmt.Sprintf("Pruned %v ZFS snapshots", zfsPruned)))
	if len(pruneErrors) > 0 {
		return errors.Join(pruneErrors...)
	}

	t.finalData = &finalData{
		zfsSnapshotName: snapName,
		bytesWritten:    bytesWritten,
		bytesTrimmed:    bytesTrimmed,
	}
	return nil
}

var _ task.Task = &ImageBackupTask{}

type SnapshotReport struct {
	Snapshots []SnapshotReportElement `json:"snapshots"`
}

type UnixTime time.Time

func (u *UnixTime) MarshalJSON() ([]byte, error) {
	if u == nil {
		return []byte("null"), nil
	}
	return []byte(strconv.FormatInt(time.Time(*u).Unix(), 10)), nil
}

var _ json.Marshaler = &UnixTime{}

type SnapshotReportInner struct {
	When   *UnixTime `json:"when"`
	Pruned bool      `json:"pruned"`
}

type SnapshotReportElement struct {
	Name     string               `json:"name"`
	Source   *SnapshotReportInner `json:"source"`
	Receiver *SnapshotReportInner `json:"receiver"`
}

func (e *SnapshotReportElement) When() time.Time {
	if e.Source == nil {
		when := time.Time(*e.Receiver.When)
		return when
	} else {
		when := time.Time(*e.Source.When)
		return when
	}
}

func (e *SnapshotReportElement) String() string {
	sb := strings.Builder{}
	sb.WriteString("Snapshot ")
	sb.WriteString(e.Name)
	sb.WriteString(": (")
	sb.WriteString(e.When().String())
	sb.WriteString("). Sender: ")
	if e.Source != nil {
		if e.Source.Pruned {
			sb.WriteString("Pruned")
		} else {
			sb.WriteString("Present")
		}
	} else {
		sb.WriteString("Absent")
	}
	sb.WriteString(", Receiver: ")
	if e.Receiver != nil {
		if e.Receiver.Pruned {
			sb.WriteString("Pruned")
		} else {
			sb.WriteString("Present")
		}
	} else {
		sb.WriteString("Absent")
	}
	return sb.String()
}

//type snapshotReportInternalComp struct {
//	Name   string
//	Source models.Snapshot
//	Rcv    models.Snapshot
//}

func (t *ImageBackupTask) makeSnapshotReport(srcSnaps []*models.CephSnapshot, srcDestroy []*models.CephSnapshot, rcvSnaps []*zfssupport.ZvolSnapshot, rcvDestroy []*zfssupport.ZvolSnapshot) *SnapshotReport {
	elements := make(map[string]*SnapshotReportElement)
	for _, snap := range srcSnaps {
		name := snap.Name()
		when := UnixTime(snap.When())
		elements[name] = &SnapshotReportElement{
			Name: name,
			Source: &SnapshotReportInner{
				When:   &when,
				Pruned: false,
			},
		}
	}
	for _, snap := range rcvSnaps {
		name := snap.Name()
		when := UnixTime(snap.When())
		rcv := &SnapshotReportInner{
			When:   &when,
			Pruned: false,
		}
		existing, found := elements[name]
		if found {
			existing.Receiver = rcv
		} else {
			elements[name] = &SnapshotReportElement{
				Name:     name,
				Receiver: rcv,
			}
		}
	}
	for _, snap := range srcDestroy {
		name := snap.Name()
		el, found := elements[name]
		if found && el.Source != nil {
			el.Source.Pruned = true
		} else {
			t.log.Warn("source snapshot mismatch! %v", name)
		}
	}
	for _, snap := range rcvDestroy {
		name := snap.Name()
		el, found := elements[name]
		if found && el.Receiver != nil {
			el.Receiver.Pruned = true
		} else {
			t.log.Warn("receiver snapshot mismatch! %v", name)
		}
	}
	out := make([]SnapshotReportElement, 0, len(elements))
	for _, el := range elements {
		out = append(out, *el)
	}
	slices.SortFunc(out, func(a, b SnapshotReportElement) int {
		return int(a.When().Unix() - b.When().Unix())
	})
	return &SnapshotReport{
		Snapshots: out,
	}
}
