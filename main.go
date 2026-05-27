package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	// 1. Setup flags
	configFlag := flag.String("config", "config.json", "Path to config JSON file")
	sshHostFlag := flag.String("ssh-host", "", "SSH Host (e.g. 1.2.3.4:22)")
	sshUserFlag := flag.String("ssh-user", "", "SSH User")
	sshKeyFlag := flag.String("ssh-key", "", "SSH Private Key Path")
	milvusAddrFlag := flag.String("milvus-addr", "", "Milvus Address (e.g. localhost:19530)")
	milvusDBFlag := flag.String("milvus-db", "", "Milvus Database")
	milvusUserFlag := flag.String("milvus-user", "", "Milvus User")
	milvusPassFlag := flag.String("milvus-pass", "", "Milvus Password")
	saveFlag := flag.Bool("save", false, "Save provided connection flags to config file")
	helpFlag := flag.Bool("h", false, "Show help")

	flag.Parse()

	if *helpFlag {
		printCliHelp()
		return
	}

	// 2. Load or initialize configuration
	cfg := DefaultConfig()
	configPath := *configFlag

	if _, err := os.Stat(configPath); err == nil {
		loadedCfg, loadErr := LoadConfig(configPath)
		if loadErr == nil {
			cfg = loadedCfg
		} else {
			fmt.Printf("⚠️  Warning loading config file: %v. Using defaults.\n", loadErr)
		}
	} else if os.IsNotExist(err) && configPath == "config.json" {
		// Create a default config file template if it does not exist
		fmt.Printf("📝 Config file 'config.json' not found. Creating a default template...\n")
		saveErr := SaveConfig("config.json", cfg)
		if saveErr != nil {
			fmt.Printf("⚠️  Failed to create default config file: %v\n", saveErr)
		}
	}

	// 3. Override configuration with flags if provided
	if *sshHostFlag != "" {
		cfg.SSHHost = *sshHostFlag
	}
	if *sshUserFlag != "" {
		cfg.SSHUser = *sshUserFlag
	}
	if *sshKeyFlag != "" {
		cfg.SSHKeyPath = ExpandPath(*sshKeyFlag)
	}
	if *milvusAddrFlag != "" {
		cfg.MilvusAddr = *milvusAddrFlag
	}
	if *milvusDBFlag != "" {
		cfg.MilvusDB = *milvusDBFlag
	}
	if *milvusUserFlag != "" {
		cfg.MilvusUser = *milvusUserFlag
	}
	if *milvusPassFlag != "" {
		cfg.MilvusPass = *milvusPassFlag
	}

	// 4. Save config if requested
	if *saveFlag {
		saveErr := SaveConfig(configPath, cfg)
		if saveErr == nil {
			fmt.Printf("💾 Config successfully saved to %s\n", configPath)
		} else {
			fmt.Printf("❌ Failed to save config: %v\n", saveErr)
		}
	}

	// 5. Establish SSH tunnel (optional)
	var sshClient *SSHClient
	var err error
	if cfg.SSHHost != "" {
		fmt.Printf("🔌 Connecting to SSH tunnel %s...\n", Colored(ColorCyan, cfg.SSHHost))
		sshClient, err = ConnectSSH(cfg)
		if err != nil {
			fmt.Printf(Colored(ColorRed, "❌ SSH Connection failed: %v\n"), err)
			os.Exit(1)
		}
		defer sshClient.Close()
		fmt.Println(Colored(ColorGreen, "✔️  SSH Tunnel connected!"))
	} else {
		fmt.Println(Colored(ColorYellow, "🔌 Skipping SSH tunnel (direct connection mode)..."))
	}

	// 6. Connect to Milvus
	fmt.Printf("⚡ Connecting to Milvus at %s (DB: %s)...\n", Colored(ColorCyan, cfg.MilvusAddr), Colored(ColorYellow, cfg.MilvusDB))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	milvusClient, err := NewMilvusClient(ctx, cfg, sshClient)
	if err != nil {
		fmt.Printf(Colored(ColorRed, "❌ Milvus Connection failed: %v\n"), err)
		os.Exit(1)
	}
	defer milvusClient.Close()
	fmt.Println(Colored(ColorGreen, "✔️  Milvus client initialized!"))
	fmt.Println()

	// 7. Check if we have CLI arguments to run single shot command
	args := flag.Args()
	if len(args) == 0 {
		// Launch interactive mode
		repl := NewREPL(cfg, sshClient, milvusClient)
		repl.Run()
	} else {
		// Execute single CLI command
		runSingleCommand(cfg, sshClient, milvusClient, args)
	}
}

func printCliHelp() {
	fmt.Println("Usage: milvus-ssh-cli [options] [command]")
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Commands (if omitted, launches interactive REPL):")
	fmt.Println("  databases / dbs                          List all databases")
	fmt.Println("  collections / colls                      List collections in current db")
	fmt.Println("  describe / desc <collection>             Describe collection schema & indexes")
	fmt.Println("  query <collection> <expr> [fields...]    Query collection fields")
	fmt.Println("  search <collection> <vec_json> [limit]   Search vectors (e.g. search coll \"[0.1, 0.2]\" 5)")
}

func runSingleCommand(cfg *Config, ssh *SSHClient, mc *MilvusClient, args []string) {
	cmd := strings.ToLower(args[0])
	cmdArgs := args[1:]
	repl := NewREPL(cfg, ssh, mc)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	switch cmd {
	case "databases", "dbs":
		err = repl.handleListDatabases(ctx)
	case "collections", "colls", "list":
		err = repl.handleListCollections(ctx)
	case "describe", "desc":
		err = repl.handleDescribeCollection(ctx, cmdArgs)
	case "query":
		err = repl.handleQuery(ctx, cmdArgs)
	case "search":
		err = repl.handleSearch(ctx, cmdArgs)
	default:
		err = fmt.Errorf("unknown command: %s", cmd)
	}

	if err != nil {
		fmt.Printf(Colored(ColorRed, "❌ Command Error: %v\n"), err)
		os.Exit(1)
	}
}
