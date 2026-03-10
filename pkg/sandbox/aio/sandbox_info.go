package aio

import "time"

type SandboxInfo struct {
	SandboxID     string    `json:"sandbox_id"`
	SandboxURL    string    `json:"sandbox_url"`
	ContainerID   string    `json:"container_id"`
	ContainerName string    `json:"container_name"`
	HostPort      int       `json:"host_port"`
	CreatedAt     time.Time `json:"created_at"`
}
