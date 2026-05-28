package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// REPL struct handles the interactive shell loop.
type REPL struct {
	cfg          *Config
	sshClient    *SSHClient
	milvusClient *MilvusClient
}

// NewREPL creates an instance of the interactive shell.
func NewREPL(cfg *Config, ssh *SSHClient, mc *MilvusClient) *REPL {
	return &REPL{
		cfg:          cfg,
		sshClient:    ssh,
		milvusClient: mc,
	}
}

// ParseArgs splits a command line into arguments, respecting double quotes.
func ParseArgs(line string) []string {
	var args []string
	var current strings.Builder
	inQuotes := false

	for _, r := range line {
		switch r {
		case '"':
			inQuotes = !inQuotes
		case ' ':
			if inQuotes {
				current.WriteRune(r)
			} else if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

// Run starts the REPL loop with autocompletion and history support.
func (r *REPL) Run() {
	completer := readline.NewPrefixCompleter(
		readline.PcItem("help"),
		readline.PcItem("?"),
		readline.PcItem("clear"),
		readline.PcItem("exit"),
		readline.PcItem("quit"),
		readline.PcItem("databases"),
		readline.PcItem("dbs"),
		readline.PcItem("use",
			readline.PcItemDynamic(r.listDatabasesAutocomplete),
		),
		readline.PcItem("collections"),
		readline.PcItem("colls"),
		readline.PcItem("list"),
		readline.PcItem("describe",
			readline.PcItemDynamic(r.listCollectionsAutocomplete),
		),
		readline.PcItem("desc",
			readline.PcItemDynamic(r.listCollectionsAutocomplete),
		),
		readline.PcItem("count",
			readline.PcItemDynamic(r.listCollectionsAutocomplete),
		),
		readline.PcItem("query",
			readline.PcItemDynamic(r.listCollectionsAutocomplete),
		),
		readline.PcItem("search",
			readline.PcItemDynamic(r.listCollectionsAutocomplete),
		),
	)

	historyFile := filepath.Join(os.TempDir(), "milvus_cli_history.tmp")
	home, err := os.UserHomeDir()
	if err == nil {
		historyFile = filepath.Join(home, ".milvus_cli_history")
	}

	prompt := func() string {
		return fmt.Sprintf("%smilvus_cli (%s)%s> ", ColorBold+ColorBlue, r.cfg.MilvusDB, ColorReset)
	}

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          prompt(),
		HistoryFile:     historyFile,
		AutoComplete:    completer,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		fmt.Printf("⚠️ Failed to initialize readline: %v. Falling back to basic reader.\n", err)
		r.runBasicReader()
		return
	}
	defer rl.Close()

	fmt.Println(Colored(ColorBold+ColorGreen, "🚀 Milvus SSH CLI Interactive Shell started."))
	fmt.Println("Type " + Colored(ColorBold+ColorCyan, "help") + " or " + Colored(ColorBold+ColorCyan, "?") + " to see available commands.")
	fmt.Println("Press " + Colored(ColorBold+ColorYellow, "TAB") + " for command and collection autocompletion.")
	fmt.Println("Press " + Colored(ColorBold+ColorYellow, "Ctrl+C") + " or type " + Colored(ColorBold+ColorYellow, "exit") + " to quit.\n")

	for {
		rl.SetPrompt(prompt())

		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				// Ctrl+C pressed - reset active line
				continue
			}
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		args := ParseArgs(line)
		cmd := strings.ToLower(args[0])
		cmdArgs := args[1:]

		switch cmd {
		case "exit", "quit":
			fmt.Println("Goodbye!")
			return
		case "clear":
			fmt.Print("\033[H\033[2J") // Clear screen ANSI sequence
		case "help", "?":
			r.printHelp()
		case "dbs", "databases":
			r.runTimed("List Databases", func(ctx context.Context) error {
				return r.handleListDatabases(ctx)
			})
		case "use":
			r.handleUse(cmdArgs)
		case "colls", "collections", "list":
			r.runTimed("List Collections", func(ctx context.Context) error {
				return r.handleListCollections(ctx)
			})
		case "desc", "describe":
			r.runTimed("Describe Collection", func(ctx context.Context) error {
				return r.handleDescribeCollection(ctx, cmdArgs)
			})
		case "query":
			r.runTimed("Query", func(ctx context.Context) error {
				return r.handleQuery(ctx, cmdArgs)
			})
		case "search":
			r.runTimed("Search", func(ctx context.Context) error {
				return r.handleSearch(ctx, cmdArgs)
			})
		case "count":
			r.runTimed("Count Entities", func(ctx context.Context) error {
				return r.handleCount(ctx, cmdArgs)
			})
		case "load":
			r.runTimed("Load Collection", func(ctx context.Context) error {
				return r.handleLoadCollection(ctx, cmdArgs)
			})
		case "release":
			r.runTimed("Release Collection", func(ctx context.Context) error {
				return r.handleReleaseCollection(ctx, cmdArgs)
			})
		default:
			fmt.Printf("⚠️  Unknown command: %s. Type 'help' for assistance.\n", cmd)
		}
		fmt.Println()
	}
}

// runBasicReader is the fallback stdin scanner if readline initialization fails.
func (r *REPL) runBasicReader() {
	reader := bufio.NewReader(os.Stdin)
	for {
		prompt := fmt.Sprintf("%smilvus-ssh (%s)%s> ", ColorBold+ColorBlue, r.cfg.MilvusDB, ColorReset)
		fmt.Print(prompt)
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		args := ParseArgs(line)
		cmd := strings.ToLower(args[0])
		cmdArgs := args[1:]

		switch cmd {
		case "exit", "quit":
			fmt.Println("Goodbye!")
			return
		case "clear":
			fmt.Print("\033[H\033[2J")
		case "help", "?":
			r.printHelp()
		case "dbs", "databases":
			r.runTimed("List Databases", func(ctx context.Context) error {
				return r.handleListDatabases(ctx)
			})
		case "use":
			r.handleUse(cmdArgs)
		case "colls", "collections", "list":
			r.runTimed("List Collections", func(ctx context.Context) error {
				return r.handleListCollections(ctx)
			})
		case "desc", "describe":
			r.runTimed("Describe Collection", func(ctx context.Context) error {
				return r.handleDescribeCollection(ctx, cmdArgs)
			})
		case "query":
			r.runTimed("Query", func(ctx context.Context) error {
				return r.handleQuery(ctx, cmdArgs)
			})
		case "search":
			r.runTimed("Search", func(ctx context.Context) error {
				return r.handleSearch(ctx, cmdArgs)
			})
		case "count":
			r.runTimed("Count Entities", func(ctx context.Context) error {
				return r.handleCount(ctx, cmdArgs)
			})
		case "load":
			r.runTimed("Load Collection", func(ctx context.Context) error {
				return r.handleLoadCollection(ctx, cmdArgs)
			})
		case "release":
			r.runTimed("Release Collection", func(ctx context.Context) error {
				return r.handleReleaseCollection(ctx, cmdArgs)
			})
		default:
			fmt.Printf("⚠️  Unknown command: %s. Type 'help' for assistance.\n", cmd)
		}
		fmt.Println()
	}
}

// runTimed measures and displays execution time of REPL commands.
func (r *REPL) runTimed(label string, fn func(context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	err := fn(ctx)
	elapsed := time.Since(start)

	if err != nil {
		fmt.Println(Colored(ColorRed, "❌ Error: ")+err.Error())
		if strings.Contains(strings.ToLower(err.Error()), "not loaded") {
			fmt.Println(Colored(ColorYellow, "💡 Tip: Use 'load <collection_name>' to load the collection into memory first."))
		}
	} else {
		fmt.Printf(Colored(ColorDim, "└─ %s completed in %v\n"), label, elapsed)
	}
}

func (r *REPL) printHelp() {
	headers := []string{"Command", "Description"}
	rows := [][]string{
		{"databases / dbs", "List all databases in the cluster"},
		{"use <db>", "Switch to another database"},
		{"collections / colls", "List all collections in the current database"},
		{"describe / desc <coll>", "Describe the schema, index state, and properties of a collection"},
		{"load <coll>", "Load a collection into memory (required before query/search)"},
		{"release <coll>", "Release a collection from memory to free resources"},
		{"count <coll>", "Get the total entity (row) count of a collection"},
		{"query <coll> <expr> [fields...]", "Query scalar fields using expression filter (e.g. query my_coll \"id in [1, 2]\" name age)"},
		{"search <coll> <vector_json> [limit] [vector_field] [fields...]", "Perform vector search (e.g. search my_coll \"[0.1, 0.2]\" 5 vec_field id label)"},
		{"clear", "Clear screen"},
		{"exit / quit", "Exit the interactive shell"},
	}
	PrintTable(headers, rows)
}

func (r *REPL) handleListDatabases(ctx context.Context) error {
	dbs, err := r.milvusClient.ListDatabases(ctx)
	if err != nil {
		return err
	}

	headers := []string{"Databases"}
	rows := make([][]string, len(dbs))
	for i, db := range dbs {
		rows[i] = []string{db}
	}
	PrintTable(headers, rows)
	return nil
}

func (r *REPL) handleUse(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: use <database_name>")
		return
	}
	dbName := args[0]
	fmt.Printf("Switching to database '%s'...\n", dbName)

	r.cfg.MilvusDB = dbName
	r.milvusClient.Close()

	mc, err := NewMilvusClient(context.Background(), r.cfg, r.sshClient)
	if err != nil {
		fmt.Printf(Colored(ColorRed, "Error: failed to connect to database '%s': %v\n"), dbName, err)
		// Try to fallback to default
		r.cfg.MilvusDB = "default"
		r.milvusClient, _ = NewMilvusClient(context.Background(), r.cfg, r.sshClient)
		return
	}
	r.milvusClient = mc
	fmt.Println(Colored(ColorGreen, "Successfully connected!"))
}

func (r *REPL) handleListCollections(ctx context.Context) error {
	colls, err := r.milvusClient.ListCollections(ctx)
	if err != nil {
		return err
	}

	headers := []string{"Collections"}
	rows := make([][]string, len(colls))
	for i, col := range colls {
		rows[i] = []string{col}
	}
	PrintTable(headers, rows)
	return nil
}

func (r *REPL) handleDescribeCollection(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return errors.New("missing collection name. Usage: describe <collection_name>")
	}
	collName := args[0]

	coll, err := r.milvusClient.DescribeCollection(ctx, collName)
	if err != nil {
		return err
	}

	fmt.Printf("%sCollection Name:%s %s\n", ColorBold+ColorCyan, ColorReset, coll.Name)
	if coll.Schema != nil {
		fmt.Printf("%sDescription:%s     %s\n", ColorBold+ColorCyan, ColorReset, coll.Schema.Description)
	} else {
		fmt.Printf("%sDescription:%s     N/A\n", ColorBold+ColorCyan, ColorReset)
	}
	fmt.Printf("%sShard Number:%s    %d\n", ColorBold+ColorCyan, ColorReset, coll.ShardNum)
	fmt.Println()

	// List Fields
	headers := []string{"Field Name", "Type", "Primary Key", "AutoID", "Params/Dimensions"}
	var rows [][]string

	for _, field := range coll.Schema.Fields {
		pk := "false"
		if field.PrimaryKey {
			pk = Colored(ColorGreen, "true")
		}
		autoID := "false"
		if field.AutoID {
			autoID = Colored(ColorGreen, "true")
		}

		// Parse type parameters (like string length or vector dimension)
		params := []string{}
		for k, v := range field.TypeParams {
			params = append(params, fmt.Sprintf("%s=%s", k, v))
		}
		paramStr := strings.Join(params, ", ")

		// Highlight field types
		fieldType := fmt.Sprintf("%v", field.DataType)
		if field.DataType == entity.FieldTypeFloatVector || field.DataType == entity.FieldTypeBinaryVector {
			fieldType = Colored(ColorYellow, fieldType)
		}

		rows = append(rows, []string{
			field.Name,
			fieldType,
			pk,
			autoID,
			paramStr,
		})
	}

	fmt.Println(Colored(ColorBold, "Schema Fields:"))
	PrintTable(headers, rows)

	// Fetch index information if possible
	fmt.Println()
	fmt.Println(Colored(ColorBold, "Indexes:"))
	idxHeaders := []string{"Field Name", "Index Type", "Metric Type"}
	var idxRows [][]string

	for _, field := range coll.Schema.Fields {
		if field.DataType == entity.FieldTypeFloatVector || field.DataType == entity.FieldTypeBinaryVector {
			indexes, err := r.milvusClient.DescribeIndex(ctx, collName, field.Name)
			if err == nil {
				for _, idx := range indexes {
					params := idx.Params()
					idxType := params["index_type"]
					metricType := params["metric_type"]
					idxRows = append(idxRows, []string{
						field.Name,
						fmt.Sprintf("%v", idxType),
						fmt.Sprintf("%v", metricType),
					})
				}
			}
		}
	}

	if len(idxRows) > 0 {
		PrintTable(idxHeaders, idxRows)
	} else {
		fmt.Println("No indexes found on vector fields.")
	}

	return nil
}

func (r *REPL) handleQuery(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return errors.New("missing collection name. Usage: query <collection_name> [expression] [fields...]")
	}

	collName := args[0]
	expr := "*"
	var outputFields []string

	if len(args) > 1 {
		expr = args[1]
		outputFields = args[2:]
	}

	rs, err := r.milvusClient.Query(ctx, collName, expr, outputFields)
	if err != nil {
		return err
	}

	if rs.Len() == 0 {
		fmt.Println("No results found.")
		return nil
	}

	// Dynamic table printing from ResultSet
	// Determine output fields if they were auto-resolved
	var fields []string
	if len(outputFields) > 0 {
		fields = outputFields
	} else {
		// If query triggered auto-resolve, we must inspect the columns returned in ResultSet
		coll, err := r.milvusClient.DescribeCollection(ctx, collName)
		if err == nil {
			for _, f := range coll.Schema.Fields {
				if f.DataType != entity.FieldTypeFloatVector && f.DataType != entity.FieldTypeBinaryVector {
					fields = append(fields, f.Name)
				}
			}
		}
	}

	// Read columns from ResultSet
	columns := make([]entity.Column, 0)
	headers := make([]string, 0)
	for _, name := range fields {
		col := rs.GetColumn(name)
		if col != nil {
			columns = append(columns, col)
			headers = append(headers, name)
		}
	}

	if len(headers) == 0 {
		return errors.New("could not extract columns from query result")
	}

	rows := make([][]string, rs.Len())
	for rowIndex := 0; rowIndex < rs.Len(); rowIndex++ {
		row := make([]string, len(columns))
		for colIndex, col := range columns {
			val, err := col.Get(rowIndex)
			if err == nil {
				row[colIndex] = formatValue(val)
			} else {
				row[colIndex] = "err"
			}
		}
		rows[rowIndex] = row
	}

	fmt.Printf("Query returned %d entities:\n", rs.Len())
	PrintTable(headers, rows)
	return nil
}

func (r *REPL) handleSearch(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return errors.New("missing parameters. Usage: search <collection_name> <vector_json> [limit] [vector_field] [fields...]")
	}

	collName := args[0]
	vectorStr := args[1]

	var vector []float32
	if strings.HasPrefix(vectorStr, "[") {
		if err := json.Unmarshal([]byte(vectorStr), &vector); err != nil {
			return fmt.Errorf("failed to parse search vector JSON: %w", err)
		}
	} else {
		if r.cfg.EmbeddingProvider == "" {
			return fmt.Errorf("search query must be a JSON array of floats (e.g. \"[0.1, 0.2]\") OR you must configure an 'embedding_provider' (e.g. 'openai' or 'ollama') in config.json to perform natural text queries")
		}
		fmt.Printf("🔄 Generating embedding for query text: \"%s\" using %s...\n", vectorStr, r.cfg.EmbeddingProvider)
		var err error
		vector, err = GetEmbedding(r.cfg, vectorStr)
		if err != nil {
			return fmt.Errorf("failed to generate embedding: %w", err)
		}
		fmt.Printf("✔️ Generated %d-dimensional embedding vector!\n", len(vector))
	}

	limit := 5
	vectorField := ""
	var outputFields []string

	if len(args) > 2 {
		val, err := strconv.Atoi(args[2])
		if err == nil {
			limit = val
		}
	}

	if len(args) > 3 {
		vectorField = args[3]
	}

	if len(args) > 4 {
		outputFields = args[4:]
	}

	results, err := r.milvusClient.Search(ctx, collName, vector, vectorField, limit, outputFields)
	if err != nil {
		return err
	}

	if len(results) == 0 || results[0].IDs.Len() == 0 {
		fmt.Println("No search results found.")
		return nil
	}

	sr := results[0]
	numRows := sr.IDs.Len()

	headers := []string{"ID", "Score / Distance"}
	// Append requested fields to headers
	for _, col := range sr.Fields {
		headers = append(headers, col.Name())
	}

	rows := make([][]string, numRows)
	for i := 0; i < numRows; i++ {
		idVal, _ := sr.IDs.Get(i)
		scoreVal := sr.Scores[i]

		row := []string{
			fmt.Sprintf("%v", idVal),
			fmt.Sprintf("%.5f", scoreVal),
		}

		// Extract output fields
		for _, col := range sr.Fields {
			val, err := col.Get(i)
			if err == nil {
				row = append(row, formatValue(val))
			} else {
				row = append(row, "err")
			}
		}
		rows[i] = row
	}

	fmt.Printf("Vector search returned %d nearest neighbors:\n", numRows)
	PrintTable(headers, rows)
	return nil
}

// handleCount retrieves the entity count of a collection.
func (r *REPL) handleCount(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return errors.New("missing collection name. Usage: count <collection_name>")
	}
	collName := args[0]

	// 1. Try count(*) query (highly precise, requires loaded collection)
	rs, err := r.milvusClient.client.Query(ctx, collName, nil, "", []string{"count(*)"})
	if err == nil && rs.Len() > 0 {
		col := rs.GetColumn("count(*)")
		if col != nil {
			val, getErr := col.Get(0)
			if getErr == nil {
				fmt.Printf("📊 Collection '%s' has %s%v%s entities.\n", collName, ColorBold+ColorGreen, val, ColorReset)
				return nil
			}
		}
	}

	// 2. Fallback to stats (estimated, works on unloaded collections too)
	stats, err := r.milvusClient.client.GetCollectionStatistics(ctx, collName)
	if err == nil {
		if rowCount, ok := stats["row_count"]; ok {
			fmt.Printf("📊 Collection '%s' has approximately %s%s%s entities (estimate from stats).\n", collName, ColorBold+ColorGreen, rowCount, ColorReset)
			return nil
		}
	}

	return fmt.Errorf("failed to retrieve count (make sure collection exists): %v", err)
}

// handleLoadCollection loads a collection into memory.
func (r *REPL) handleLoadCollection(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return errors.New("missing collection name. Usage: load <collection_name>")
	}
	collName := args[0]
	fmt.Printf("Loading collection '%s' into memory (blocking wait)...\n", collName)
	err := r.milvusClient.client.LoadCollection(ctx, collName, false)
	if err != nil {
		return err
	}
	fmt.Println(Colored(ColorGreen, "✔️  Collection loaded successfully!"))
	return nil
}

// handleReleaseCollection releases a collection from memory.
func (r *REPL) handleReleaseCollection(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return errors.New("missing collection name. Usage: release <collection_name>")
	}
	collName := args[0]
	fmt.Printf("Releasing collection '%s' from memory...\n", collName)
	err := r.milvusClient.client.ReleaseCollection(ctx, collName)
	if err != nil {
		return err
	}
	fmt.Println(Colored(ColorGreen, "✔️  Collection released successfully!"))
	return nil
}


// listCollectionsAutocomplete dynamically lists collection names for autocompletion.
func (r *REPL) listCollectionsAutocomplete(line string) []string {
	if r.milvusClient == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	colls, err := r.milvusClient.ListCollections(ctx)
	if err != nil {
		return nil
	}
	return colls
}

// listDatabasesAutocomplete dynamically lists database names for autocompletion.
func (r *REPL) listDatabasesAutocomplete(line string) []string {
	if r.milvusClient == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	dbs, err := r.milvusClient.ListDatabases(ctx)
	if err != nil {
		return nil
	}
	return dbs
}

// formatValue converts any field value to a string, truncating vectors to keep terminal tables clean.
func formatValue(val interface{}) string {
	switch v := val.(type) {
	case []float32:
		if len(v) > 2 {
			return fmt.Sprintf("[%.4f, %.4f, ... (%d dims)]", v[0], v[1], len(v))
		}
		return fmt.Sprintf("%v", v)
	default:
		return fmt.Sprintf("%v", val)
	}
}



