# Milvus SSH CLI

A lightweight, terminal-based interactive query client and administration tool for Milvus databases, featuring direct gRPC-over-SSH tunneling, dynamic tab-autocompletion, persistent command history, and custom table visualization.

---

## ✨ Features

- **Direct gRPC-over-SSH Tunneling**: Custom transport dialing directly routes gRPC traffic inside the SSH channel. No external port-forwarding setups, socket leaks, or local port conflicts.
- **Stateful REPL Interactive Shell**: Launch a terminal prompt that keeps connection channels warm. Run sequential queries instantly without connection overhead.
- **Smart Wildcard Querying**: Query all collection rows simply using `query <collection_name>` (simulating `SELECT * FROM t`). The client automatically inspects the schema and resolves the primary key type under the hood.
- **Dynamic Tab Autocompletion**:
  - Auto-suggests commands.
  - Dynamically fetches and suggests Milvus **databases** (for `use <db>`) and **collections** (for `describe`, `query`, `search`, etc.) over the SSH tunnel.
- **Command History**: Navigate history with `Up` and `Down` arrow keys. Command history is persisted across shell sessions in `~/.milvus_cli_history`.
- **Entity Count**: Run `count <collection>` to get precise entity counts (on loaded collections) or metadata-based estimates (on unloaded collections).
- **Collection Control**: Dynamically load or release collections from memory using `load` and `release`.
- **Pretty Print Tables**: Custom-built visualizer formats schemas, indexes, and queries into beautifully structured Unicode-bordered tables.

---

## 🛠️ System Architecture

```
┌───────────────┐                  ┌───────────────┐                 ┌─────────────────────┐
│ Local Machine │                  │ SSH Jump Host │                 │  Internal Network   │
│               │  gRPC over SSH   │               │ Direct TCP Dial │                     │
│  [milvus-ssh] ├─────────────────►│   [sshd]      ├────────────────►│ [Milvus gRPC:19530] │
└───────────────┘  (Channel Dial)  └───────────────┘                 └─────────────────────┘
```

The Go application initializes an SSH client and registers a custom `grpc.WithContextDialer` that routes connections through `sshClient.DialContext(ctx, milvusAddr)`. All transport is securely tunneled.

---

## 🚀 Quick Start

### 1. Prerequisites
- **Go**: Version `1.23.10` or newer is recommended.

### 2. Installation
You can download pre-compiled cross-platform binaries directly from the **Releases** tab on GitHub.

Alternatively, compile from source:
```bash
git clone https://github.com/yourusername/milvus_cli.git
cd milvus_cli
go build -o milvus_cli
```

### 3. Configure Connection
Copy the example configuration file and fill in your SSH jump host credentials and Milvus target details:
* **Global config (Recommended)**: Create a directory named `.milvus_cli` in your user home folder and put `config.json` there:
  ```bash
  mkdir -p ~/.milvus_cli
  cp config.example.json ~/.milvus_cli/config.json
  # Edit ~/.milvus_cli/config.json with your credentials
  ```
* **Local config**: Alternatively, copy it to the current directory:
  ```bash
  cp config.example.json config.json
  # Edit config.json with your credentials
  ```

### 4. Run the Client
```bash
# Enter interactive shell mode
./milvus_cli

# Or run a single-shot command directly and exit
./milvus_cli query my_collection "id != ''" name age
```

---

## ⚙️ Configuration Schema (`config.json`)

| Parameter | Description |
| :--- | :--- |
| `ssh_host` | SSH Server IP/Host and Port (e.g. `180.184.70.87:22` or `127.0.0.1:29646`). Leave empty if connecting directly to Milvus. |
| `ssh_user` | SSH Username (e.g., `root`, `ubuntu`). |
| `ssh_password`| SSH Password (optional, prompts securely if empty and key auth is not used). |
| `ssh_key_path`| Path to SSH private key (optional, supports `~` expansion, default `~/.ssh/id_rsa`). |
| `ssh_key_pass`| Passphrase for encrypted private key (optional, prompts securely if needed). |
| `milvus_addr` | Milvus server IP/Domain and Port (relative to the SSH host, e.g. `localhost:19530`). Auto-sanitizes `http://` prefixes. |
| `milvus_user` | Milvus Database Username (optional). |
| `milvus_pass` | Milvus Database Password (optional). |
| `milvus_db`   | Target Database Name (optional, defaults to `default`). |
| `embedding_provider` | Text embedding provider. Supported: `"openai"` (or OpenAI-compatible APIs like Volcengine/DashScope/DeepSeek) and `"ollama"` (local offline). Set to `""` to disable. |
| `embedding_api_key` | API Key for embedding model (optional, required for OpenAI). |
| `embedding_model` | Embedding model to use (default: `"text-embedding-3-small"` for OpenAI, e.g., `"mxbai-embed-large"` for Ollama). |
| `embedding_api_url` | Custom URL endpoint for embedding requests (optional, e.g., `"http://localhost:11434"` for Ollama). |

---

## 🧠 Natural Language Semantic Search

If `embedding_provider` is configured, you don't need to generate and copy-paste long float vectors to search! 

The `search` command automatically detects if the input is a text query instead of a JSON float array:
```bash
# 1. Search using raw float vector (always supported)
search my_collection "[0.12, -0.4, 0.8]"

# 2. Search using natural text (requires embedding_provider configured)
search my_collection "how to resolve order abnormalities"
```

The CLI will connect to your configured provider, generate the float vector from your query string, and execute the similarity search in Milvus.

---

## 📖 Command Reference (REPL Mode)

| Command | Syntax / Example | Description |
| :--- | :--- | :--- |
| **databases** / **dbs** | `databases` | List all databases |
| **use** | `use my_db` | Switch target database (reconnects within active SSH tunnel) |
| **collections** / **colls** | `collections` | List all collections in active database |
| **describe** / **desc** | `desc my_collection` | Show collection fields, primary keys, and index details |
| **load** | `load my_collection` | Load collection into memory (required before query/search) |
| **release** | `release my_collection` | Release collection from memory to free cluster resources |
| **count** | `count my_collection` | Retrieve accurate or estimated entity row count |
| **query** | `query my_collection` <br>`query my_collection "id != ''" name age` | Query fields (supports wildcard `*` or empty expression to match all) |
| **search** | `search my_collection "text query"` <br>`search my_collection "[0.1, -0.2]" 5` | Run vector similarity searches using text queries or raw float vectors |
| **clear** | `clear` | Clears terminal screen |
| **exit** / **quit** | `exit` | Disconnects SSH and Milvus clients and exits |

---

## 🔒 Security Notice

**Never commit your `config.json` containing active passwords or server IPs to git.**
A `.gitignore` file is included in this repository to prevent tracking of local `config.json` configurations. Always use the template `config.example.json` to publish repository settings changes.

## 📄 License
This project is licensed under the MIT License.
