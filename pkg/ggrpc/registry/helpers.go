package registry

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// marshalInstance serialises an Instance to JSON.
func marshalInstance(inst *Instance) (string, error) {
	b, err := json.Marshal(inst)
	if err != nil {
		return "", fmt.Errorf("registry: marshal instance: %w", err)
	}
	return string(b), nil
}

// unmarshalInstance deserialises an Instance from JSON.
func unmarshalInstance(s string) (*Instance, error) {
	var inst Instance
	if err := json.Unmarshal([]byte(s), &inst); err != nil {
		return nil, fmt.Errorf("registry: unmarshal instance: %w", err)
	}
	return &inst, nil
}

// splitHostPort splits "host:port" and returns (host, port, error).
func splitHostPort(address string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid port %q: %w", portStr, err)
	}
	return host, port, nil
}

// labelsToTags converts a label map to a sorted slice of "key=value" strings.
func labelsToTags(labels map[string]string) []string {
	tags := make([]string, 0, len(labels))
	for k, v := range labels {
		tags = append(tags, k+"="+v)
	}
	return tags
}

// tagsToLabels converts "key=value" tags back to a map.
func tagsToLabels(tags []string) map[string]string {
	m := make(map[string]string, len(tags))
	for _, t := range tags {
		parts := strings.SplitN(t, "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}
