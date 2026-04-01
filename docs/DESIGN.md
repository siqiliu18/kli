# kli — Design Document

> Developer-facing. For user documentation see `README.md` (written after v1 ships).

---

## 1. Problem Statement

`kubectl` outputs raw, unformatted text with no progress indication, no color coding, and no summary. `kli` wraps the Kubernetes API with production-grade UX: spinners during operations, color-coded per-resource results, clean summaries, and readable log output.

---

## 2. Scope — v1

Exactly 4 commands. No more for v1.

| Command | Purpose |
|---|---|
| `kli apply -f <file\|folder>` | Apply resources with progress + color output |
| `kli undeploy -f <file\|folder>` | Delete resources with per-resource results |
| `kli status [namespace]` | Show deployment health + all pods per resource |
| `kli logs <pod> [flags]` | Stream formatted, filterable pod logs |

Pod discovery for `kli logs` is intentionally handled by `kli status`, which lists individual pod names under each resource. A `kli logs deployment/<name>` shorthand is deferred to v2.

---

## 3. Architecture

```
kcli/
  main.go

  cmd/
    root.go        # root cobra command, global flags (--namespace / -n)
    apply.go       # kli apply
    undeploy.go    # kli undeploy
    status.go      # kli status
    logs.go        # kli logs

  internal/
    k8s/
      client.go    # build *dynamic.Interface from ~/.kube/config
      apply.go     # apply resources, return []ResourceResult
      undeploy.go  # delete resources, return []ResourceResult
      status.go    # fetch deployments/statefulsets/daemonsets + their pods
      logs.go      # stream pod logs, optional grep filter
    ui/
      spinner.go   # wrap charmbracelet spinner
      table.go     # formatted table output (lipgloss)
      colors.go    # color scheme constants

  docs/
    DESIGN.md      # this file
  README.md        # user-facing (written post-v1)
```

### Layering rules

- `cmd/` owns CLI parsing and calls `internal/k8s/` and `internal/ui/`. It knows nothing about the Kubernetes API directly.
- `internal/k8s/` owns all Kubernetes API calls. It returns plain Go structs — no terminal output.
- `internal/ui/` owns all terminal rendering. It takes plain Go structs — no Kubernetes types.
- No circular imports between `internal/` packages.

---

## 4. Tech Stack

| Component | Library | Reason |
|---|---|---|
| Language | Go | Standard for k8s tooling; static binary |
| Kubernetes client | `k8s.io/client-go/dynamic` | Dynamic client handles any resource kind without codegen |
| CLI framework | `github.com/spf13/cobra` | Standard Go CLI framework (used by kubectl itself) |
| Terminal UI | `github.com/charmbracelet/lipgloss` | Terminal styling |
| Spinner | Custom goroutine + braille frames | Zero deps; race-free stop via done channel |
| YAML parsing | `k8s.io/apimachinery/pkg/util/yaml` | Correctly splits multi-document YAML |
| Kubeconfig | `k8s.io/client-go/tools/clientcmd` | Reads `~/.kube/config`, respects `KUBECONFIG` env var |

---

## 5. Command Design

### 5.1 `kli apply -f <file|folder>`

**Flow:**
1. Walk directory recursively — collect all `.yaml` / `.yml` files via `filepath.Walk`
2. Split each file on `---` using `k8s.io/apimachinery/pkg/util/yaml`
3. Decode each document into `unstructured.Unstructured`
4. Apply via server-side apply (SSA): `dynamic.Resource(gvr).Namespace(ns).Apply(...)`
5. Collect `[]ResourceResult{name, kind, action, err}` per resource
6. Render: spinner during apply → per-resource color row → summary line

**Namespace confirmation** (when `-n` is omitted):
```
No namespace specified. Deploy to "default"? [y/N]:
```
Saying `n` exits with a `kubectl create namespace` hint. Skipped entirely when `-n` is passed — CI/script friendly.

**Namespace validation** (all commands): `NamespaceExists()` is called before any operation. Returns a clear error if the namespace doesn't exist rather than cryptic API errors or silent empty results.

**Flags:** `--namespace / -n`, `--dry-run`

**Exit code:** non-zero if any resource failed (CI-friendly).

**Output legend:** `✅ created` / `🔄 configured` / `⚡ unchanged` / `❌ failed`

---

### 5.2 `kli undeploy -f <file|folder>`

Same file-walking and YAML-splitting logic as `apply`.

**Differences from apply:**
- Calls `Delete` instead of `Apply`
- Resources not found → skip with `⚡ not found (skipped)`, not an error
- Resources with finalizers → warn with `⚠️`, do not hang silently
- Prints `Namespace: <name>` header before results (no prompt — user is expected to pass `-n` explicitly)
- `--force` (v2 only): strips finalizers before deleting

**Output legend:** `🗑️ deleted` / `⚡ skipped` / `⚠️ warning`

---

### 5.3 `kli status [namespace]`

Lists deployments, statefulsets, and daemonsets. Under each resource, lists all individual pods — this is the intended way users discover pod names for `kli logs`.

**Example output:**
```
Namespace: aspera

DEPLOYMENTS
  ✅  api-server          3/3  Running
      api-server-abc123        Running
      api-server-def456        Running
      api-server-ghi789        Running
  ⚠️  frontend            1/3  Degraded
      frontend-ccc333          Running
      frontend-ddd444          Pending
      frontend-eee555          CrashLoopBackOff
  ❌  db-migrator         0/1  Failed
      db-migrator-fff666       Error
```

**Color coding:**
- Green (`✅`) — all pods ready
- Yellow (`⚠️`) — some pods not ready (degraded)
- Red (`❌`) — zero pods ready or resource in error

**API calls needed:**
- `AppsV1().Deployments(ns).List()`
- `AppsV1().StatefulSets(ns).List()`
- `AppsV1().DaemonSets(ns).List()`
- `CoreV1().Pods(ns).List()` — filtered by owner reference to group pods under their parent

---

### 5.4 `kli logs <pod> [flags]`

**Flags:** `--follow / -f`, `--grep <pattern>`, `--namespace / -n`, `--container / -c` (when pod has multiple containers)

**Behavior:**
- Tab completion queries live pod names from the cluster
- Adds formatted timestamps to each log line
- `--grep` filters lines client-side (post-stream)
- Color-codes log levels: `INFO` (white), `WARN` (yellow), `ERROR` (red)

**v2 note:** Accept `deployment/<name>` to auto-select a running pod.

---

## 6. Kubernetes Client Setup

### What is `~/.kube/config`?

`~/.kube/config` is a YAML file that stores everything a CLI needs to connect to a Kubernetes cluster:

- **Where** — the API server URL (e.g. `https://my-cluster:6443`)
- **Who** — your credentials (certificate, token, or username/password)
- **Which** — your current context (active cluster + namespace)

It is created/populated when you first authenticate to a cluster — e.g. `oc login` for OCP, `aws eks update-kubeconfig` for EKS. Installing `kubectl` via Homebrew only provides the binary; the config file comes from authenticating to an actual cluster.

Without it, `kli` has no idea what cluster to talk to or how to authenticate — same as `kubectl` failing with `"no configuration has been provided"`.

### Connection flow

```
~/.kube/config
      ↓
clientcmd.BuildConfigFromFlags()   // parses kubeconfig → *rest.Config (server URL + auth)
      ↓
dynamic.NewForConfig(config)       // builds HTTP client → dynamic.Interface
      ↓
Kubernetes API server calls
```

`rest.Config` is the low-level struct holding the server URL and credentials. You don't use it directly — you pass it to client constructors. We return it alongside the dynamic client because `kli logs` needs a separate typed client (`kubernetes.Interface`) for log streaming, which also takes a `rest.Config`.

### Why two clients?

| Client | Type | Used for |
|---|---|---|
| Dynamic client | `dynamic.Interface` | apply, undeploy, status — works with any resource kind |
| Typed client | `kubernetes.Interface` | logs — log streaming is a special HTTP call only on the typed client |

```go
// internal/k8s/client.go
type Client struct {
    typeClient    kubernetes.Interface  // status, logs, namespace validation
    dynamicClient dynamic.Interface     // apply, undeploy — any resource kind
}

func NewClient() (*Client, error) {
    loadingRules := clientcmd.NewDefaultClientConfigLoadingRules() // respects KUBECONFIG
    config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
        loadingRules, &clientcmd.ConfigOverrides{},
    ).ClientConfig()
    ...
}

// Called at the start of every command before doing any work.
func (c *Client) NamespaceExists(namespace string) error {
    _, err := c.typeClient.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
    if errors.IsNotFound(err) {
        return fmt.Errorf("namespace %q not found — create it first: kubectl create namespace %s", namespace, namespace)
    }
    return err
}
```

`dynamic.Interface` handles apply/undeploy for any resource kind (CRDs included) without generating typed clients. `kubernetes.Interface` is used for everything that needs typed structs: status, logs, and namespace validation.

---

## 7. Error Handling Philosophy

- **User errors** (bad file path, unknown namespace): print readable message, exit 1. No stack traces.
- **Namespace not found**: caught upfront by `NamespaceExists()` before any operation — clear message with `kubectl create namespace` hint rather than cryptic API errors or silent empty results.
- **Partial failures** (one resource fails during apply): continue applying the rest, report the failure in the summary, exit non-zero.
- **Kubernetes API errors**: surface the message from the API response — don't wrap with generic text.
- **kubeconfig not found**: fail fast with a clear message pointing to `~/.kube/config`.

---

## 8. Definition of Done (v1)

- [x] `kli apply -f file.yaml` works
- [x] `kli apply -f folder/` recursively applies all YAML files
- [x] `kli apply` handles multi-document YAML (`---` separator)
- [x] `kli undeploy -f` mirrors apply behavior for deletion
- [x] `kli status` shows deployment health + individual pod names per resource
- [x] `kli logs` streams formatted logs with `--follow` and `--grep`
- [x] Color-coded output with spinner during apply/undeploy
- [x] `--namespace / -n` flag on all commands
- [x] `--dry-run` on apply
- [x] `--container / -c` on logs (multi-container pods)
- [x] Namespace confirmation prompt on `apply` when `-n` omitted
- [x] Namespace validation on all commands — clear error if namespace not found
- [x] Unit tests (`go test ./...`) — k8s helpers + UI rendering + fake client integration
- [x] GitHub Actions: `go test` + `go build` + `go vet` on push/PR
- [x] Tested on real k8s cluster (namespace kli1, kli2)
- [x] README with CI badge
- [x] README demo GIF (asciinema)

---

## 9. Out of Scope (v1)

- `kli get` / `kli describe` — use `kubectl` for raw resource inspection
- `kli logs deployment/<name>` — v2
- `--force` finalizer removal on undeploy — v2
- Multi-cluster / context switching — v2
- Plugin system — not planned
