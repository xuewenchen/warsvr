package main

import (
	"cardwar/pkg/auth"
	"cardwar/pkg/conf"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	if cmd == "list" || cmd == "type" || cmd == "port" || cmd == "jwt" || cmd == "status" || cmd == "conns" {
		handleQuery(os.Args)
		return
	}

	// service commands: build, start, stop, restart, reboot
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: svchelper %s <cs-1|gw-1|all> [config.yml]\n", cmd)
		os.Exit(1)
	}

	configPath := "config.yml"
	target := os.Args[2]
	if len(os.Args) > 3 {
		configPath = os.Args[3]
	}

	if err := conf.Load(configPath); err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	switch cmd {
	case "build":
		doBuild(target)
	case "start":
		doStart(target, configPath)
	case "stop":
		doStop(target)
	case "restart":
		doStop(target)
		time.Sleep(1 * time.Second)
		doStart(target, configPath)
	case "reboot":
		doStop(target)
		doBuild(target)
		doStart(target, configPath)
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("Usage: svchelper <cmd> <instance|all> [config.yml]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  build <xxx>     compile binary")
	fmt.Println("  start <xxx>     run binary")
	fmt.Println("  stop <xxx>      kill process by port")
	fmt.Println("  restart <xxx>   stop + start")
	fmt.Println("  reboot <xxx>    stop + build + start")
	fmt.Println("  status          show full cluster topology & connections")
	fmt.Println("  list [config]   show all instances")
	fmt.Println("  type <id>       get service type for instance")
	fmt.Println("  port <id>       get listen port for instance")
	fmt.Println("  jwt <playerId>  generate JWT token for player")
}

// ---- query commands ----

func handleQuery(args []string) {
	configPath := "config.yml"
	// Load config for all query commands
	if len(args) >= 3 && strings.HasSuffix(args[len(args)-1], ".yml") {
		configPath = args[len(args)-1]
	}
	if err := conf.Load(configPath); err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	switch args[1] {
	case "list":
		for svcName, nodes := range conf.GlobalConfig.Services {
			for _, n := range nodes {
				if n.ID != "" {
					fmt.Printf("%s %s\n", n.ID, svcName)
				}
			}
		}
	case "type":
		if len(args) < 3 {
			os.Exit(1)
		}
		svc, _ := findServiceNode(args[2])
		if svc == "" {
			os.Exit(1)
		}
		fmt.Println(svc)
	case "port":
		if len(args) < 3 {
			os.Exit(1)
		}
		_, node := findServiceNode(args[2])
		if node == nil {
			os.Exit(1)
		}
		_, p := parseHostPort(listenAddr(node))
		fmt.Println(p)

	case "jwt":
		if len(args) < 3 {
			os.Exit(1)
		}
		playerID, _ := strconv.ParseInt(args[2], 10, 64)
		secret := conf.GlobalConfig.Gateway.JWTSecret
		token, err := auth.GenerateJWT(playerID, secret)
		if err != nil {
			fmt.Fprintf(os.Stderr, "jwt error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(token)

	case "status", "conns":
		showStatus()
	}
}

// ---- service commands ----

func doBuild(target string) {
	if target == "all" {
		for _, svc := range discoverServices() {
			buildOne(svc)
		}
		buildSelf()
		return
	}
	// Try as service name directly
	for _, svc := range discoverServices() {
		if target == svc {
			buildOne(svc)
			return
		}
	}
	// Try as instance ID (look up in config)
	svc, _ := findServiceNode(target)
	if svc == "" {
		fmt.Fprintf(os.Stderr, "ERROR: %q is not a service or instance\n", target)
		fmt.Fprintf(os.Stderr, "  Services: %s\n", strings.Join(discoverServices(), ", "))
		os.Exit(1)
	}
	buildOne(svc)
}

func discoverServices() []string {
	entries, err := os.ReadDir("apps")
	if err != nil {
		return nil
	}
	var svcs []string
	for _, e := range entries {
		if e.IsDir() {
			if _, err := os.Stat("apps/" + e.Name() + "/cmd"); err == nil {
				svcs = append(svcs, e.Name())
			}
		}
	}
	return svcs
}

func buildSelf() {
	fmt.Printf(">>> Building svchelper...\n")
	cmd := exec.Command("go", "build", "-o", fmt.Sprintf("bin/svchelper%s", exeExt()), "./tools/svchelper/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "build svchelper failed: %v\n", err)
	}
	fmt.Printf("  -> bin/svchelper%s\n", exeExt())
}

func buildOne(svc string) {
	fmt.Printf(">>> Building %s...\n", svc)
	cmd := exec.Command("go", "build", "-o", fmt.Sprintf("bin/%s%s", svc, exeExt()), "./apps/"+svc+"/cmd/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  -> bin/%s%s\n", svc, exeExt())
}

func doStart(target, configPath string) {
	if target == "all" {
		fmt.Printf("Starting all from %s...\n", configPath)
		for _, inst := range listInstances() {
			startOne(inst.svc, inst.id, inst.port, configPath)
			time.Sleep(1 * time.Second)
		}
		return
	}
	svc, node := findServiceNode(target)
	if svc == "" {
		fmt.Fprintf(os.Stderr, "ERROR: instance %q not found in config\n", target)
		os.Exit(1)
	}
	_, port := parseHostPort(listenAddr(node))
	startOne(svc, target, port, configPath)
}

func startOne(svc, id string, port int, configPath string) {
	if portOccupied(port) {
		fmt.Printf("  %s (%s) is already running on port %d\n", svc, id, port)
		return
	}

	bin := "bin/" + svc + exeExt()
	if _, err := os.Stat(bin); os.IsNotExist(err) {
		buildOne(svc)
	}

	logName := svc
	if id != "" {
		logName = svc + "-" + id
	}
	os.MkdirAll("log", 0755)
	logPath := "log/" + logName + ".log"

	fmt.Printf(">>> Starting %s (conf=%s, id=%s)...\n", svc, configPath, id)

	args := []string{"-conf", configPath}
	if id != "" {
		args = append(args, "-id", id)
	}

	logFile, _ := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	cmd := exec.Command(bin, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "  FAILED to start: %v\n", err)
		os.Exit(1)
	}

	// Poll port until listening (Dial may wait for downstream connections)
	for i := 0; i < 20; i++ {
		time.Sleep(400 * time.Millisecond)
		if portOccupied(port) {
			fmt.Printf("  OK (pid:%d, port:%d, log:%s)\n", cmd.Process.Pid, port, logPath)
			pidDir := "bin/.pids"
			os.MkdirAll(pidDir, 0755)
			os.WriteFile(pidDir+"/"+svc+".pid", []byte(strconv.Itoa(cmd.Process.Pid)), 0644)
			return
		}
	}
	fmt.Printf("  FAILED - port %d not listening after 8s. Tail log:\n", port)
	cmd.Process.Kill()
	printLogTail(logPath)
	os.Exit(1)
}

func doStop(target string) {
	if target == "all" {
		for _, inst := range listInstances() {
			killByPort(inst.port, inst.svc)
		}
		return
	}
	svc, node := findServiceNode(target)
	if svc == "" {
		fmt.Fprintf(os.Stderr, "ERROR: instance %q not found in config\n", target)
		os.Exit(1)
	}
	_, port := parseHostPort(listenAddr(node))
	killByPort(port, svc)
}

func killByPort(port int, svc string) {
	fmt.Printf(">>> Stopping %s (port %d)...\n", svc, port)
	pid := findPIDByPort(port)
	if pid > 0 {
		p, _ := os.FindProcess(pid)
		p.Kill()
		time.Sleep(300 * time.Millisecond)
		fmt.Println("  Stopped")
	} else {
		fmt.Println("  Not running")
	}
}

// ---- helpers ----

type instance struct {
	svc  string
	id   string
	port int
}

func listInstances() []instance {
	var list []instance
	for svcName, nodes := range conf.GlobalConfig.Services {
		for _, n := range nodes {
			if n.ID != "" {
				_, p := parseHostPort(listenAddr(&n))
				list = append(list, instance{svc: svcName, id: n.ID, port: p})
			}
		}
	}
	return list
}

func listenAddr(n *conf.ServerNode) string {
	if n.WSListen != "" {
		return n.WSListen // gateway: client-facing port
	}
	return n.Listen // chatsvr & others
}

func findServiceNode(id string) (string, *conf.ServerNode) {
	for svcName, nodes := range conf.GlobalConfig.Services {
		for i := range nodes {
			if nodes[i].ID == id {
				return svcName, &nodes[i]
			}
		}
	}
	return "", nil
}

func parseHostPort(addr string) (string, int) {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		return "", 0
	}
	port, _ := strconv.Atoi(parts[1])
	return parts[0], port
}

// portOccupied checks if a TCP port is currently listening.
func portOccupied(port int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "netstat", "-ano")
	out, _ := cmd.Output()
	return strings.Contains(string(out), fmt.Sprintf(":%d ", port)) &&
		strings.Contains(string(out), "LISTENING")
}

// findPIDByPort returns the PID listening on a port.
func findPIDByPort(port int) int {
	cmd := exec.Command("netstat", "-ano")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, fmt.Sprintf(":%d ", port)) && strings.Contains(line, "LISTENING") {
			fields := strings.Fields(line)
			if len(fields) >= 5 {
				pid, _ := strconv.Atoi(fields[len(fields)-1])
				return pid
			}
		}
	}
	return 0
}

func showStatus() {
	out, _ := exec.Command("netstat", "-ano").Output()
	nets := strings.Split(string(out), "\n")

	fmt.Println()
	fmt.Println("=== Gateway Instances ===")
	for _, inst := range listInstancesOf("gateway") {
		pid := findPIDByPort(inst.wsPort)
		status := "DOWN"
		if pid > 0 {
			status = fmt.Sprintf("RUNNING (pid:%d)", pid)
		}
		fmt.Printf("  %s  ws:%d  tcp:%d  %s\n", inst.id, inst.wsPort, inst.tcpPort, status)
		if pid > 0 {
			for _, cs := range listInstancesOf("chatsvr") {
				state := connState(nets, pid, cs.port)
				fmt.Printf("    ├── %s  :%d  %s\n", cs.id, cs.port, state)
			}
		}
	}

	fmt.Println()
	fmt.Println("=== Backend Instances ===")
	for svc, list := range groupByService() {
		if svc == "gateway" {
			continue
		}
		title := svc
		if len(title) > 0 {
			title = strings.ToUpper(title[:1]) + title[1:]
		}
		fmt.Printf("%s:\n", title)
		for _, inst := range list {
			pid := findPIDByPort(inst.port)
			gwCount := 0
			for _, gw := range listInstancesOf("gateway") {
				gwPID := findPIDByPort(gw.wsPort)
				if gwPID > 0 && connState(nets, gwPID, inst.port) == "ESTABLISHED" {
					gwCount++
				}
			}
			status := "DOWN"
			if pid > 0 {
				status = fmt.Sprintf("RUNNING (pid:%d, %d gateway(s))", pid, gwCount)
			}
			fmt.Printf("  %s  :%d  %s\n", inst.id, inst.port, status)
		}
	}
}

type svcInstance struct {
	id      string
	svc     string
	port    int
	wsPort  int
	tcpPort int
}

func listInstancesOf(svc string) []svcInstance {
	var list []svcInstance
	nodes := conf.GlobalConfig.Services[svc]
	for _, n := range nodes {
		inst := svcInstance{id: n.ID, svc: svc}
		if svc == "gateway" {
			_, inst.wsPort = parseHostPort(n.WSListen)
			_, inst.tcpPort = parseHostPort(n.TCPListen)
		} else {
			_, inst.port = parseHostPort(n.Listen)
		}
		list = append(list, inst)
	}
	return list
}

func groupByService() map[string][]svcInstance {
	m := make(map[string][]svcInstance)
	for svc := range conf.GlobalConfig.Services {
		if svc == "gateway" {
			continue
		}
		m[svc] = listInstancesOf(svc)
	}
	return m
}

func connState(nets []string, pid, port int) string {
	for _, line := range nets {
		if strings.Contains(line, fmt.Sprintf(":%d", port)) &&
			strings.Contains(line, "ESTABLISHED") &&
			strings.Contains(line, strconv.Itoa(pid)) {
			return "ESTABLISHED"
		}
	}
	return "NOT CONNECTED"
}

func printLogTail(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	start := 0
	if len(lines) > 6 {
		start = len(lines) - 6
	}
	for _, line := range lines[start:] {
		fmt.Println(line)
	}
}

func exeExt() string {
	if os.PathSeparator == '\\' {
		return ".exe"
	}
	return ""
}
