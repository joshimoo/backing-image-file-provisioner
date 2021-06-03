package server

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"

	"github.com/longhorn/backing-image-file-provisioner/api"
	"github.com/longhorn/backing-image-file-provisioner/pkg/types"
	"github.com/longhorn/backing-image-file-provisioner/pkg/util"
)

const (
	ProgressUpdateInterval = time.Second
)

type ProgressUpdater interface {
	UpdateProgress(size int64)
}

type FileProvisioner struct {
	ctx     context.Context
	errorCh chan error
	lock    *sync.RWMutex
	log     logrus.FieldLogger

	diskUUID   string
	sourceType types.DataSourceType
	parameters map[string]string

	fileName      string
	filePath      string
	state         types.FileState
	size          int64
	progress      int
	processedSize int64
	md5Checksum   string
	message       string
}

func LaunchFileProvisioner(ctx context.Context, errorCh chan error,
	fileName, sourceType string, parameters map[string]string) (*FileProvisioner, error) {
	diskUUID, err := util.GetDiskConfig(types.DiskPathInContainer)
	if err != nil {
		return nil, err
	}
	workDir := filepath.Join(types.DiskPathInContainer, types.FileProvisionerDirectoryName)
	if err := os.Mkdir(workDir, 0666); err != nil && !os.IsExist(err) {
		return nil, err
	}

	fp := &FileProvisioner{
		ctx:     ctx,
		errorCh: errorCh,
		lock:    &sync.RWMutex{},
		log: logrus.StandardLogger().WithFields(
			logrus.Fields{
				"component": "file-provisioner",
			},
		),

		diskUUID:   diskUUID,
		sourceType: types.DataSourceType(sourceType),
		parameters: parameters,

		fileName: fileName,
		filePath: filepath.Join(workDir, fileName),
		state:    types.FileStateStarting,
	}

	if err := fp.init(); err != nil {
		return nil, err
	}

	return fp, nil
}

func (fp *FileProvisioner) init() error {
	switch fp.sourceType {
	case types.DataSourceTypeURL:
		return fp.downloadFromURL(fp.parameters)
	case types.DataSourceTypeUpload:
	default:
		return fmt.Errorf("unknown data source type: %v", fp.sourceType)

	}
	return nil
}

func (fp *FileProvisioner) UpdateProgress(processedSize int64) {
	fp.lock.Lock()
	defer fp.lock.Unlock()

	if fp.state == types.FileStateStarting {
		fp.state = types.FileStateInProgress
	}
	if fp.state == types.FileStateReady {
		return
	}

	fp.processedSize = fp.processedSize + processedSize
	if fp.size > 0 {
		fp.progress = int((float32(fp.processedSize) / float32(fp.size)) * 100)
	}
}

func (fp *FileProvisioner) finishProvisioning(err error) {
	defer func() {
		if err != nil {
			fp.lock.Lock()
			fp.state = types.FileStateFailed
			fp.message = err.Error()
			fp.lock.Unlock()

			fp.errorCh <- err
		}
	}()

	if err != nil {
		err = errors.Wrapf(err, "failed to finish file %v provisioning", fp.filePath)
		return
	}

	stat, statErr := os.Stat(fp.filePath)
	if statErr != nil {
		err = errors.Wrapf(err, "failed to stat file %v after file provisioning", fp.filePath)
		return
	}

	md5Checksum, cksumErr := util.GetFileMD5Checksum(fp.filePath)
	if cksumErr != nil {
		err = errors.Wrapf(cksumErr, "failed to calculate md5 checksum for file %v after file provisioning", fp.filePath)
		return
	}

	fp.lock.Lock()
	fp.size = stat.Size()
	fp.processedSize = stat.Size()
	fp.progress = 100
	fp.state = types.FileStateReady
	fp.md5Checksum = md5Checksum
	fp.lock.Unlock()

	return
}

func (fp *FileProvisioner) downloadFromURL(parameters map[string]string) error {
	url := parameters[types.DataSourceTypeURLParameterURL]
	if url == "" {
		return fmt.Errorf("no URL for file downloading")
	}

	size, err := GetDownloadSize(url)
	if err != nil {
		return err
	}
	if size > 0 {
		fp.lock.Lock()
		fp.size = size
		fp.lock.Unlock()
	}

	go func() {
		_, err := DownloadFile(fp.ctx, url, fp.filePath, fp)
		fp.finishProvisioning(err)
	}()

	return nil
}

func GetDownloadSize(url string) (fileSize int64, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), types.HTTPTimeout)
	defer cancel()

	rr, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	client := http.Client{}
	resp, err := client.Do(rr)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("expected status code 200 from %s, got %s", url, resp.Status)
	}

	if resp.Header.Get("Content-Length") == "" {
		// -1 indicates unknown size
		fileSize = -1
	} else {
		fileSize, err = strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
		if err != nil {
			return 0, err
		}
	}

	return fileSize, nil
}

func DownloadFile(ctx context.Context, url, filepath string, updater ProgressUpdater) (written int64, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	rr, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	client := http.Client{}
	resp, err := client.Do(rr)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("expected status code 200 from %s, got %s", url, resp.Status)
	}

	outFile, err := os.Create(filepath)
	if err != nil {
		return 0, err
	}
	defer outFile.Close()

	return idleTimeoutCopy(ctx, cancel, resp.Body, outFile, updater)
}

func idleTimeoutCopy(ctx context.Context, cancel context.CancelFunc, src io.Reader, dst io.Writer, updater ProgressUpdater) (written int64, err error) {
	writeCh := make(chan int)

	go func() {
		t := time.NewTimer(types.HTTPTimeout)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				cancel()
			case <-writeCh:
				if !t.Stop() {
					<-t.C
				}
				t.Reset(types.HTTPTimeout)
			}
		}
	}()

	buf := make([]byte, 512*1024)
	for {
		// Read will error out once the context is cancelled.
		nr, readErr := src.Read(buf)
		if nr > 0 {
			nw, writeErr := dst.Write(buf[0:nr])
			if writeErr != nil {
				err = writeErr
				break
			}
			writeCh <- nw
			written += int64(nw)
			updater.UpdateProgress(int64(nw))
		}
		if readErr != nil {
			if readErr != io.EOF {
				err = readErr
			}
			break
		}
	}
	return written, err
}

func (fp *FileProvisioner) Upload(writer http.ResponseWriter, request *http.Request) {
	err := fp.doUpload(request)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (fp *FileProvisioner) doUpload(request *http.Request) error {
	fp.log.Debugf("FileProvisioner receives request Upload")

	if fp.sourceType != types.DataSourceTypeUpload {
		return fmt.Errorf("cannot do upload since data source type is %v rather than upload", fp.sourceType)
	}

	ctx, cancelFunc := context.WithCancel(fp.ctx)
	defer cancelFunc()

	queryParams := request.URL.Query()
	size, err := strconv.ParseInt(queryParams.Get("size"), 10, 64)
	if err != nil {
		return err
	}
	if size%types.DefaultSectorSize != 0 {
		return fmt.Errorf("the uploaded file size %d should be a multiple of %d bytes since Longhorn uses directIO by default", size, types.DefaultSectorSize)
	}
	fp.lock.Lock()
	fp.size = size
	fp.lock.Unlock()

	fp.log.Infof("request.ContentLength %v", request.ContentLength)

	reader, err := request.MultipartReader()
	if err != nil {
		return err
	}
	var p *multipart.Part
	for {
		if p, err = reader.NextPart(); err != nil {
			return err
		}
		if p.FormName() != "chunk" {
			fp.log.Warnf("Unexpected form %v in upload request, will ignore it", p.FormName())
			continue
		}
		break
	}
	if p == nil || p.FormName() != "chunk" {
		return fmt.Errorf("cannot get the uploaded data since the upload request doesn't contain form 'chunk'")
	}

	if err := os.Remove(fp.filePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := os.OpenFile(fp.filePath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = f.Truncate(size); err != nil {
		return err
	}

	// Since there is no way to directly track the progress of io.Copy(),
	// We will periodically check the file size instead.
	// In the unit test, the uploading is too fast to trigger the first
	// file progress update.
	go func() {
		var st syscall.Stat_t
		t := time.NewTicker(ProgressUpdateInterval)
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := syscall.Stat(fp.filePath, &st); err != nil {
					fp.log.Warnf("Fail to get actual size of file: %v", err)
				} else {
					fp.lock.Lock()
					increasedSize := st.Blocks*types.DefaultSectorSize - fp.processedSize
					fp.lock.Unlock()
					fp.UpdateProgress(increasedSize)
				}
			}
		}

	}()

	defer fp.finishProvisioning(err)
	if _, err = io.Copy(f, p); err != nil {
		if err != io.EOF {
			err = errors.Wrapf(err, "failed to write data to file %v during uploading", fp.filePath)
			return err
		}
		err = nil
	}

	return nil
}

func (fp *FileProvisioner) Get(writer http.ResponseWriter, request *http.Request) {
	fp.log.Debugf("FileProvisioner receives request Get")

	fp.lock.RLock()
	fp.lock.RUnlock()
	fi := api.FileInfo{
		DiskUUID:   fp.diskUUID,
		SourceType: string(fp.sourceType),
		Parameters: fp.parameters,

		FileName:      fp.fileName,
		FilePath:      fp.filePath,
		State:         string(fp.state),
		Size:          fp.size,
		Progress:      fp.progress,
		ProcessedSize: fp.processedSize,
		MD5Checksum:   fp.md5Checksum,
	}

	outgoingJSON, err := json.Marshal(fi)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.Write(outgoingJSON)
}

func (fp *FileProvisioner) Close(writer http.ResponseWriter, request *http.Request) {
	fp.log.Debugf("FileProvisioner receives request Close")

	if f, ok := writer.(http.Flusher); ok {
		f.Flush()
	}
	fp.doClose(request)
}

func (fp *FileProvisioner) doClose(request *http.Request) {
	fp.lock.RLock()
	state := fp.state
	fp.lock.RUnlock()

	if state != types.FileStateReady {
		fp.errorCh <- fmt.Errorf("closing the server while the file provisioning is still in progress")
		return
	}
	fp.errorCh <- nil
	return
}
