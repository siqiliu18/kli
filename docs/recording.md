# Recording a Demo GIF

Use `asciinema` to record and `agg` to convert to GIF.

## Setup

```bash
brew install asciinema
brew install agg
```

## Record

Open iTerm2 or Terminal.app (not VS Code integrated terminal — it produces a black GIF).

```bash
asciinema rec demo.cast (or asciinema rec --command bash demo.cast)
```

Run the demo commands:

```bash
# verify cluster is ready (Rancher Desktop or Docker Desktop)
kubectl cluster-info
kubectl get nodes

# demo
go build -o kli .
./kli apply -f testdata/ -n kli2
./kli status -n kli2
./kli logs <pod-from-status-output> -n kli2 --grep nginx
```

Stop recording with `ctrl+d` (or type `exit`).

## Convert to GIF

```bash
agg --theme monokai demo.cast demo.gif
```

> Use `--theme monokai` to avoid the black screen issue caused by VS Code terminal's dark background theme.

## Add to README

```markdown
![demo](demo.gif)
```
