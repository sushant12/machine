package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	_ "github.com/sushant12/machine/docs"

	"github.com/firecracker-microvm/firecracker-go-sdk/vsock"
	"github.com/gorilla/mux"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"github.com/sushant12/machine/pkg/rootfs"
	httpSwagger "github.com/swaggo/http-swagger"
	_ "github.com/swaggo/swag"
)

type VMConfig struct {
	Config struct {
		Init struct {
			Exec []string `json:"exec"`
		} `json:"init"`
		AutoDestroy bool   `json:"auto_destroy"`
		Image       string `json:"image"`
		Files       []struct {
			GuestPath string `json:"guest_path"`
			RawValue  string `json:"raw_value"`
		} `json:"files"`
		Guest struct {
			CPUs     int    `json:"cpus"`
			MemoryMB int    `json:"memory_mb"`
		} `json:"guest"`
	} `json:"config"`
}

type ExecCommand struct {
	Cmd []string `json:"cmd"`
}

type ExecResponse struct {
	Output string `json:"output"`
}

type SysInfo struct {
	Memory       Memory                `json:"memory"`
	LoadAverage  [3]float32            `json:"load_average"`
	CPUs         map[int]CPU           `json:"cpus"`
	Disks        []DiskStat            `json:"disks"`
	Net          []NetworkDevice       `json:"net"`
	FileFD       FileFD                `json:"filefd"`
}

type Memory struct {
	MemTotal      uint64  `json:"mem_total"`
	MemFree       uint64  `json:"mem_free"`
	MemAvailable  *uint64 `json:"mem_available,omitempty"`
	Buffers       uint64  `json:"buffers"`
	Cached        uint64  `json:"cached"`
	SwapCached    uint64  `json:"swap_cached"`
	Active        uint64  `json:"active"`
	Inactive      uint64  `json:"inactive"`
	SwapTotal     uint64  `json:"swap_total"`
	SwapFree      uint64  `json:"swap_free"`
	Dirty         uint64  `json:"dirty"`
	Writeback     uint64  `json:"writeback"`
	Slab          uint64  `json:"slab"`
	Shmem         *uint64 `json:"shmem,omitempty"`
	VmallocTotal  uint64  `json:"vmalloc_total"`
	VmallocUsed   uint64  `json:"vmalloc_used"`
	VmallocChunk  uint64  `json:"vmalloc_chunk"`
}

type CPU struct {
	User       float32  `json:"user"`
	Nice       float32  `json:"nice"`
	System     float32  `json:"system"`
	Idle       float32  `json:"idle"`
	IOWait     *float32 `json:"iowait,omitempty"`
	IRQ        *float32 `json:"irq,omitempty"`
	SoftIRQ    *float32 `json:"softirq,omitempty"`
	Steal      *float32 `json:"steal,omitempty"`
	Guest      *float32 `json:"guest,omitempty"`
	GuestNice  *float32 `json:"guest_nice,omitempty"`
}

type DiskStat struct {
	Name             string `json:"name"`
	ReadsCompleted   uint64 `json:"reads_completed"`
	ReadsMerged      uint64 `json:"reads_merged"`
	SectorsRead      uint64 `json:"sectors_read"`
	TimeReading      uint64 `json:"time_reading"`
	WritesCompleted  uint64 `json:"writes_completed"`
	WritesMerged     uint64 `json:"writes_merged"`
	SectorsWritten   uint64 `json:"sectors_written"`
	TimeWriting      uint64 `json:"time_writing"`
	IOInProgress     uint64 `json:"io_in_progress"`
	TimeIO           uint64 `json:"time_io"`
	TimeIOWeighted   uint64 `json:"time_io_weighted"`
}

type NetworkDevice struct {
	Name            string `json:"name"`
	RecvBytes       uint64 `json:"recv_bytes"`
	RecvPackets     uint64 `json:"recv_packets"`
	RecvErrs        uint64 `json:"recv_errs"`
	RecvDrop        uint64 `json:"recv_drop"`
	RecvFifo        uint64 `json:"recv_fifo"`
	RecvFrame       uint64 `json:"recv_frame"`
	RecvCompressed  uint64 `json:"recv_compressed"`
	RecvMulticast   uint64 `json:"recv_multicast"`
	SentBytes       uint64 `json:"sent_bytes"`
	SentPackets     uint64 `json:"sent_packets"`
	SentErrs        uint64 `json:"sent_errs"`
	SentDrop        uint64 `json:"sent_drop"`
	SentFifo        uint64 `json:"sent_fifo"`
	SentColls       uint64 `json:"sent_colls"`
	SentCarrier     uint64 `json:"sent_carrier"`
	SentCompressed  uint64 `json:"sent_compressed"`
}

type FileFD struct {
	Allocated uint64 `json:"allocated"`
	Maximum   uint64 `json:"maximum"`
}

type VMStatus struct {
	OK bool `json:"ok"`
}

type CreateResponse struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

var vsockPath string

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		logrus.WithError(err).Errorf("Command failed: %s %v\nStdout: %s\nStderr: %s", name, args, stdout.String(), stderr.String())
		return fmt.Errorf("%s: %s", err, stderr.String())
	}
	logrus.Infof("Command succeeded: %s %v\nStdout: %s", name, args, stdout.String())
	return nil
}

func generateNanoID() (string, error) {
	return gonanoid.Generate("0123456789", 7)
}

func generateMACAddress() string {
	// Use 02 as the first octet to ensure it's a locally administered address
	id, _ := generateNanoID()
	return fmt.Sprintf("02:00:%02x:%02x:%02x:%02x", 
		id[0]&0xFF, id[1]&0xFF, id[2]&0xFF, id[3]&0xFF)
}

func getTapDeviceName(machineID string) string {
	return fmt.Sprintf("tap%s", machineID)
}

func createConfigFile(vmConfig VMConfig, rootfsPath, vsockPath, configFilePath string) error {
	machineID := filepath.Base(rootfsPath)
	
	config := map[string]interface{}{
		"boot-source": map[string]interface{}{
			"kernel_image_path": "./bin/vmlinux",
			"boot_args":         "console=ttyS0 reboot=k panic=1 pci=off init=/firestarter/init",
			"initrd_path":       nil,
		},
		"drives": []map[string]interface{}{
			{
				"drive_id":       "init",
				"is_root_device": true,
				"is_read_only":   false,
				"path_on_host":   "/home/sush/Documents/machine/"+ rootfsPath + "/tmpinit", 
			},
			{
				"drive_id":       "rootfs",
				"is_root_device": false,
				"is_read_only":   false,
				"path_on_host":   "/home/sush/Documents/machine/" + rootfsPath+"/rootfs.ext4",
			},
		},
		"machine-config": map[string]interface{}{
			"vcpu_count":        vmConfig.Config.Guest.CPUs,
			"mem_size_mib":      vmConfig.Config.Guest.MemoryMB,
			"smt":               false,
			"track_dirty_pages": false,
			"huge_pages":        "None",
		},
		"network-interfaces": []map[string]interface{}{
			{
				"iface_id":      "eth0",
				"guest_mac":     generateMACAddress(),
				"host_dev_name": getTapDeviceName(machineID),
			},
		},
		"vsock": map[string]interface{}{
			"guest_cid": 3,
			"uds_path":  vsockPath,
		},
	}

	configFile, err := os.Create(configFilePath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer configFile.Close()

	if err := json.NewEncoder(configFile).Encode(config); err != nil {
		return fmt.Errorf("failed to encode config file: %w", err)
	}

	return nil
}

func createRunJSON(vmConfig VMConfig, machineDir string) error {
	runConfig := map[string]interface{}{
		"ImageConfig": map[string]interface{}{
			"Entrypoint": nil,
			"Cmd":        []string{"/bin/sleep", "inf"},
			"Env":        []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
			"WorkingDir": "/",
			"User":       "nobody",
		},
		"ExecOverride": nil,
		"ExtraEnv":     nil,
		"UserOverride": nil,
		"CmdOverride":  nil,
		"IPConfigs": []map[string]interface{}{
			{
				"Gateway": "172.17.0.1/24",
				"IP":      "172.17.0.2/24",
				"Mask":    24,
			},
		},
		"Tty":      true,
		"Hostname": "container-1",
		"Mounts":   nil,
		"RootDevice": nil,
		"EtcResolv": map[string]interface{}{
			"Nameservers": []string{"8.8.8.8", "8.8.4.4"},
		},
		"EtcHosts": []map[string]interface{}{
			{
				"Host": "localhost",
				"IP":   "127.0.0.1",
				"Desc": "Local loopback",
			},
			{
				"Host": "container-1",
				"IP":   "172.17.0.2",
				"Desc": "Container hostname",
			},
		},
		"files": func() []map[string]string {
			files := []map[string]string{}
			for _, file := range vmConfig.Config.Files {
				files = append(files, map[string]string{
					"guest_path": file.GuestPath,
					"raw_value":  file.RawValue,
				})
			}
			return files
		}(),
	}

	runJSONPath := filepath.Join(machineDir, "run.json")
	runJSONFile, err := os.Create(runJSONPath)
	if err != nil {
		return fmt.Errorf("failed to create run.json file: %w", err)
	}
	defer runJSONFile.Close()

	if err := json.NewEncoder(runJSONFile).Encode(runConfig); err != nil {
		return fmt.Errorf("failed to encode run.json file: %w", err)
	}

	return nil
}

func setupTmpInitDevice(machineDir, binDir, runJSONPath string) error {
	tmpInitPath := filepath.Join(machineDir, "tmpinit")
	initMountPath := filepath.Join(machineDir, "initmount")
	initBinaryPath := filepath.Join(binDir, "init")

	logrus.Info("Setting up tmpinit device...")

	if err := exec.Command("fallocate", "-l", "64M", tmpInitPath).Run(); err != nil {
		return fmt.Errorf("failed to allocate tmpinit file: %w", err)
	}

	if err := exec.Command("mkfs.ext2", tmpInitPath).Run(); err != nil {
		return fmt.Errorf("failed to format tmpinit file: %w", err)
	}

	if err := os.MkdirAll(initMountPath, 0755); err != nil {
		return fmt.Errorf("failed to create initmount directory: %w", err)
	}

	_ = exec.Command("sudo", "umount", initMountPath).Run()

	if err := exec.Command("sudo", "mount", "-o", "loop,noatime", tmpInitPath, initMountPath).Run(); err != nil {
		return fmt.Errorf("failed to mount tmpinit file: %w", err)
	}

	initDir := filepath.Join(initMountPath, "firestarter")
	if err := os.MkdirAll(initDir, 0755); err != nil {
		return fmt.Errorf("failed to create /firestarter directory: %w", err)
	}

	if err := exec.Command("sudo", "cp", initBinaryPath, filepath.Join(initDir, "init")).Run(); err != nil {
		return fmt.Errorf("failed to copy init binary: %w", err)
	}

	if err := exec.Command("sudo", "cp", runJSONPath, filepath.Join(initDir, "run.json")).Run(); err != nil {
		return fmt.Errorf("failed to copy run.json file: %w", err)
	}

	if err := exec.Command("sudo", "umount", initMountPath).Run(); err != nil {
		return fmt.Errorf("failed to unmount tmpinit file: %w", err)
	}

	if err := os.RemoveAll(initMountPath); err != nil {
		return fmt.Errorf("failed to remove initmount directory: %w", err)
	}

	logrus.Info("tmpinit device setup completed.")
	return nil
}

func startFirecrackerInstance(vmConfig VMConfig, rootfsPath, socketPath, vsockPath, configFilePath string) error {
	if err := createConfigFile(vmConfig, rootfsPath, vsockPath, configFilePath); err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}

	logrus.Info("Starting Firecracker process...")
	cmd := exec.Command("sudo", "./bin/firecracker", "--api-sock", socketPath, "--config-file", configFilePath, "--log-path", "./"+ rootfsPath+"/firecracker.log", "--level", "Debug", "--show-level", "--show-log-origin")
	logrus.Infof("Executing command: %s %v", cmd.Path, cmd.Args)
	// Redirect Firecracker logs to the console
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start firecracker process: %w", err)
	}
	logrus.Info("Firecracker process started.")

	return nil
}

func communicateWithVsock(vsockPath string, execCmd ExecCommand) (string, error) {
	conn, err := vsock.Dial(vsockPath, 10000)
	if err != nil {
		return "", fmt.Errorf("failed to connect to vsock: %w", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	cmdJSON, err := json.Marshal(execCmd)
	if err != nil {
		return "", fmt.Errorf("failed to marshal exec command: %w", err)
	}
	postRequest := fmt.Sprintf("POST /v1/exec HTTP/1.1\r\n"+
		"Host: 3:10000\r\n"+
		"Content-Type: application/json\r\n"+
		"Content-Length: %d\r\n"+
		"\r\n"+
		"%s\r\n", len(cmdJSON), cmdJSON)
	if _, err := conn.Write([]byte(postRequest)); err != nil {
		return "", fmt.Errorf("failed to send POST request: %w", err)
	}

	_, body, err := readHttpResponse(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read POST response: %w", err)
	}
	logrus.Infof("POST response: %s", body)

	return body, nil
}

func sendVsockRequest(vsockPath, endpoint string) (string, error) {
	conn, err := vsock.Dial(vsockPath, 10000)
	if err != nil {
		return "", fmt.Errorf("failed to connect to vsock: %w", err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	getRequest := fmt.Sprintf("GET %s HTTP/1.1\r\n"+
		"Host: 3:10000\r\n"+
		"Content-Type: application/json\r\n"+
		"\r\n", endpoint)
	if _, err := conn.Write([]byte(getRequest)); err != nil {
		return "", fmt.Errorf("failed to send GET request: %w", err)
	}

	_, body, err := readHttpResponse(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read GET response: %w", err)
	}
	logrus.Infof("GET response from %s: %s", endpoint, body)

	return body, nil
}

func getVMStatus(vsockPath string) (string, error) {
	return sendVsockRequest(vsockPath, "/v1/status")
}

func getSystemInfo(vsockPath string) (string, error) {
	return sendVsockRequest(vsockPath, "/v1/sysinfo")
}

func readHttpResponse(reader *bufio.Reader) (string, string, error) {
	var headers bytes.Buffer
	var contentLength int

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", "", err
		}
		
		headers.WriteString(line)
		
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				length, err := strconv.Atoi(strings.TrimSpace(parts[1]))
				if err == nil {
					contentLength = length
				}
			}
		}
		
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	var body string
	if contentLength > 0 {
		bodyBytes := make([]byte, contentLength)
		_, err := io.ReadFull(reader, bodyBytes)
		if err != nil {
			return headers.String(), "", err
		}
		body = string(bodyBytes)
	} else {
		bodyBytes, err := io.ReadAll(reader)
		if err != nil {
			return headers.String(), "", err
		}
		body = string(bodyBytes)
	}

	return headers.String(), body, nil
}

func createExt4Image(machineDir, outputPath string) error {
	cmd := exec.Command("./bin/mkext4", "-input", machineDir, "-output", outputPath, "-size", "2048")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		logrus.WithError(err).Errorf("Failed to create ext4 image: %s", stderr.String())
		return fmt.Errorf("failed to create ext4 image: %w", err)
	}
	logrus.Infof("Successfully created ext4 image: %s", stdout.String())
	return nil
}

// @Summary Start a new Firecracker VM
// @Description Starts a new Firecracker VM with the provided configuration
// @Accept json
// @Produce json
// @Param vmConfig body VMConfig true "VM Configuration"
// @Success 200 {object} CreateResponse "VM Creation Response"
// @Failure 400 {string} string "Bad Request"
// @Failure 500 {string} string "Internal Server Error"
// @Router /create [post]
func startVMHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var vmConfig VMConfig
	if err := json.NewDecoder(r.Body).Decode(&vmConfig); err != nil {
		logrus.WithError(err).Error("Failed to decode JSON")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	machineID, err := generateNanoID()
	if err != nil {
		logrus.WithError(err).Error("Failed to generate machine ID")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	machineDir := filepath.Join(".", machineID)
	if err := os.MkdirAll(machineDir, 0755); err != nil {
		logrus.WithError(err).Error("Failed to create machine directory")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	socketPath := filepath.Join("/tmp", fmt.Sprintf("firecracker-%s.socket", machineID))
	vsockPath = filepath.Join("/tmp", fmt.Sprintf("firecracker-vsock-%s.sock", machineID))
	configFilePath := filepath.Join("/tmp", fmt.Sprintf("firecracker-config-%s.json", machineID))
	logPath := filepath.Join(machineDir, "firecracker.log")

	// logrus.Info("copying tmpinit...")
	// if err := utils.CopyFile("./bin/tmpinit", filepath.Join(machineDir, "tmpinit")); err != nil {
	// 	logrus.WithError(err).Error("Failed to copy tmpinit file")
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	return
	// }
	logrus.Info("create logfile...")
	if _, err := os.Create(logPath); err != nil {
		logrus.WithError(err).Error("Failed to create log file")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go func() {
		logrus.Info("extracting rootfs...")

		if err := rootfs.ExtractFromImage(vmConfig.Config.Image, machineDir+"/rootfs"); err != nil {
			logrus.WithError(err).Error("Failed to extract rootfs")
			return
		}
		logrus.Info("create ext4...")

		if err := createExt4Image(machineDir+"/rootfs", machineDir+"/rootfs.ext4"); err != nil {
			logrus.WithError(err).Error("Failed to create ext4 image")
			return
		}

		rootfsDir := filepath.Join(machineDir, "rootfs")
		if err := os.RemoveAll(rootfsDir); err != nil {
			logrus.WithError(err).Error("Failed to clean up rootfs directory")
		}

		if err := createRunJSON(vmConfig, machineDir); err != nil {
			logrus.WithError(err).Error("Failed to create run.json file")
			return
		}

		runJSONPath := filepath.Join(machineDir, "run.json")
		if err := setupTmpInitDevice(machineDir, "./bin", runJSONPath); err != nil {
			logrus.WithError(err).Error("Failed to set up tmpinit device")
			return
		}

		if err := startFirecrackerInstance(vmConfig, machineDir, socketPath, vsockPath, configFilePath); err != nil {
			logrus.WithError(err).Error("Failed to start Firecracker instance")
			return
		}

		logrus.Infof("VM started with config: %+v", vmConfig)
		logrus.Infof("vsockPath: %s", vsockPath)
	}()

	response := CreateResponse{
		ID:    machineID,
		State: "created",
	}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		logrus.WithError(err).Error("Failed to marshal response JSON")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)
}

// @Summary Execute a command in a VM
// @Description Executes a command in a running VM
// @Accept json
// @Produce json
// @Param machine_id path string true "Machine ID"
// @Param execCmd body ExecCommand true "Command to execute"
// @Success 200 {object} ExecResponse "Command Output"
// @Failure 400 {string} string "Bad Request"
// @Failure 500 {string} string "Internal Server Error"
// @Router /exec/{machine_id} [post]
func execCommandHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	machineID := vars["machine_id"]
	vsockPath := filepath.Join("/tmp", fmt.Sprintf("firecracker-vsock-%s.sock", machineID))

	var execCmd ExecCommand
	if err := json.NewDecoder(r.Body).Decode(&execCmd); err != nil {
		logrus.WithError(err).Error("Failed to decode JSON")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response, err := communicateWithVsock(vsockPath, execCmd)
	if err != nil {
		logrus.WithError(err).Error("Failed to communicate with vsock")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	responseJSON, err := json.Marshal(ExecResponse{Output: response})
	if err != nil {
		logrus.WithError(err).Error("Failed to marshal response JSON")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)
}

// @Summary Get VM Status
// @Description Retrieves the status of a running VM
// @Accept json
// @Produce json
// @Param machine_id path string true "Machine ID"
// @Success 200 {object} VMStatus "VM Status"
// @Failure 400 {string} string "Bad Request"
// @Failure 500 {string} string "Internal Server Error"
// @Router /status/{machine_id} [get]
func vmStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	machineID := vars["machine_id"]
	vsockPath := filepath.Join("/tmp", fmt.Sprintf("firecracker-vsock-%s.sock", machineID))

	body, err := getVMStatus(vsockPath)
	if err != nil {
		logrus.WithError(err).Error("Failed to communicate with vsock")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var vmStatus VMStatus
	if err := json.Unmarshal([]byte(body), &vmStatus); err != nil {
		logrus.WithError(err).Error("Failed to unmarshal status response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	responseJSON, err := json.Marshal(vmStatus)
	if err != nil {
		logrus.WithError(err).Error("Failed to marshal response JSON")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)
}

// @Summary Get System Information
// @Description Retrieves system information of a running VM
// @Accept json
// @Produce json
// @Param machine_id path string true "Machine ID"
// @Success 200 {object} SysInfo "System Information"
// @Failure 400 {string} string "Bad Request"
// @Failure 500 {string} string "Internal Server Error"
// @Router /sys_info/{machine_id} [get]
func sysInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	machineID := vars["machine_id"]
	vsockPath := filepath.Join("/tmp", fmt.Sprintf("firecracker-vsock-%s.sock", machineID))

	body, err := getSystemInfo(vsockPath)
	if err != nil {
		logrus.WithError(err).Error("Failed to communicate with vsock")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var sysInfo SysInfo
	if err := json.Unmarshal([]byte(body), &sysInfo); err != nil {
		logrus.WithError(err).Error("Failed to unmarshal sysinfo response")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	responseJSON, err := json.Marshal(sysInfo)
	if err != nil {
		logrus.WithError(err).Error("Failed to marshal response JSON")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(responseJSON)
}
 
// @title Firecracker VM API
// @version 1.0
// @description API for managing and controlling Firecracker VMs
// @host localhost:8080
// @BasePath /
func main() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.SetLevel(logrus.InfoLevel)

	r := mux.NewRouter()
	r.HandleFunc("/create", startVMHandler).Methods("POST")
	r.HandleFunc("/status/{machine_id}", vmStatus).Methods("GET")
	r.HandleFunc("/sys_info/{machine_id}", sysInfo).Methods("GET")
	r.HandleFunc("/exec/{machine_id}", execCommandHandler).Methods("POST")
	
	r.PathPrefix("/swagger/").Handler(httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
		httpSwagger.DeepLinking(true),
		httpSwagger.DocExpansion("none"),
		httpSwagger.DomID("swagger-ui"),
	))

	logrus.Info("Server is listening on port 8080...")
	logrus.Info("Swagger documentation available at http://localhost:8080/swagger/")
	if err := http.ListenAndServe(":8080", r); err != nil {
		logrus.WithError(err).Fatal("Failed to start server")
	}
}
