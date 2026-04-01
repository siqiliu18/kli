package k8s

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/siqiliu/kli/internal/ui"
	corev1 "k8s.io/api/core/v1"
)

// Logs streams logs from pod in namespace to stdout.
// If follow is true, the stream stays open and tails new log lines (like tail -f).
// If grep is non-empty, only lines containing that substring are printed (client-side filter).
// If container is non-empty, logs are fetched from that specific container — required
// when a pod has multiple containers, otherwise the API server returns an error.
func (c *Client) Logs(pod, namespace, container, grep string, follow bool) error {
	opts := &corev1.PodLogOptions{
		Timestamps: true,
		// Follow keeps the HTTP connection open and streams new lines as they arrive.
		// When false, the API server returns the current log snapshot and closes.
		Follow: follow,
		// Container is optional for single-container pods. For multi-container pods
		// the API server requires it — without it the request is rejected.
		Container: container,
	}
	req := c.typeClient.CoreV1().Pods(namespace).GetLogs(pod, opts)
	stream, err := req.Stream(context.Background())
	if err != nil {
		return err
	}
	defer stream.Close()

	reader := bufio.NewReader(stream)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			// grep is a client-side filter applied after receiving each line.
			// The full log stream is still fetched from the server — only
			// the printed output is filtered.
			if grep == "" || strings.Contains(line, grep) {
				fmt.Println(ui.ColorLogLine(strings.TrimRight(line, "\n")))
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}
	return nil
}
