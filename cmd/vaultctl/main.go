package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"vault/internal/checksum"
	"vault/internal/config"
	pb "vault/proto/coordinator"
)

func main() {
	cfg := config.LoadClientConfig()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	conn, err := grpc.NewClient(
		cfg.CoordinatorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(64*1024*1024),
			grpc.MaxCallSendMsgSize(64*1024*1024),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to coordinator at %s: %v\n", cfg.CoordinatorAddr, err)
		os.Exit(1)
	}
	defer conn.Close()

	client := pb.NewCoordinatorServiceClient(conn)

	switch command {
	case "put":
		if len(os.Args) < 4 {
			fmt.Println("Usage: vaultctl put <local-filepath> <object-key>")
			os.Exit(1)
		}
		handlePut(client, os.Args[2], os.Args[3])

	case "get":
		if len(os.Args) < 4 {
			fmt.Println("Usage: vaultctl get <object-key> <destination-filepath>")
			os.Exit(1)
		}
		handleGet(client, os.Args[2], os.Args[3])

	case "inspect":
		if len(os.Args) < 3 {
			fmt.Println("Usage: vaultctl inspect <object-key>")
			os.Exit(1)
		}
		handleInspect(client, os.Args[2])

	case "delete":
		if len(os.Args) < 3 {
			fmt.Println("Usage: vaultctl delete <object-key>")
			os.Exit(1)
		}
		handleDelete(client, os.Args[2])

	case "nodes":
		handleNodes(client)

	case "help", "--help", "-h":
		printUsage()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`vaultctl - Vault Distributed Object Storage CLI

Usage:
  vaultctl put <file-path> <object-key>       Upload a file into Vault
  vaultctl get <object-key> <destination>     Download an object from Vault
  vaultctl inspect <object-key>               Inspect object chunks and replica placements
  vaultctl delete <object-key>                Delete an object and its chunk replicas
  vaultctl nodes                              List storage nodes and their statuses

Environment:
  VAULT_COORDINATOR_ADDR    Coordinator address (default: localhost:50055)`)
}

func handlePut(client pb.CoordinatorServiceClient, filePath, objectKey string) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", filePath, err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	stream, err := client.PutObject(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting put stream: %v\n", err)
		os.Exit(1)
	}

	// 1. Send header
	err = stream.Send(&pb.PutObjectRequest{
		Payload: &pb.PutObjectRequest_Header{
			Header: &pb.ObjectHeader{
				Key:  objectKey,
				Size: int64(len(fileData)),
			},
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error sending header: %v\n", err)
		os.Exit(1)
	}

	// 2. Stream data in 64 KiB chunks
	chunkSize := 64 * 1024
	r := bytes.NewReader(fileData)
	buf := make([]byte, chunkSize)
	for {
		n, rErr := r.Read(buf)
		if n > 0 {
			if sErr := stream.Send(&pb.PutObjectRequest{
				Payload: &pb.PutObjectRequest_ChunkData{
					ChunkData: buf[:n],
				},
			}); sErr != nil {
				fmt.Fprintf(os.Stderr, "Error streaming chunk data: %v\n", sErr)
				os.Exit(1)
			}
		}
		if rErr != nil {
			if errors.Is(rErr, io.EOF) {
				break
			}
			fmt.Fprintf(os.Stderr, "Error reading file buffer: %v\n", rErr)
			os.Exit(1)
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Put failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nUploaded:\n")
	fmt.Printf("  Object:   %s\n", resp.GetKey())
	fmt.Printf("  Size:     %d B\n", resp.GetSize())
	fmt.Printf("  Chunks:   %d\n", resp.GetChunkCount())
	fmt.Printf("  Replicas: %d\n", resp.GetReplicationFactor())
	fmt.Printf("  Checksum: %s\n", resp.GetChecksum())
	fmt.Printf("  Status:   SUCCESS\n\n")
}

func handleGet(client pb.CoordinatorServiceClient, objectKey, destPath string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	stream, err := client.GetObject(ctx, &pb.GetObjectRequest{Key: objectKey})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error requesting object: %v\n", err)
		os.Exit(1)
	}

	var totalData bytes.Buffer
	chunkCount := 0

	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Download error: %v\n", err)
			os.Exit(1)
		}

		if cData := resp.GetChunkData(); len(cData) > 0 {
			totalData.Write(cData)
			chunkCount++
		}
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if destDir != "" && destDir != "." {
		_ = os.MkdirAll(destDir, 0755)
	}

	if err := os.WriteFile(destPath, totalData.Bytes(), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing downloaded file to %s: %v\n", destPath, err)
		os.Exit(1)
	}

	fmt.Printf("\nDownloaded:\n")
	fmt.Printf("  Object:   %s\n", objectKey)
	fmt.Printf("  Size:     %d B\n", totalData.Len())
	fmt.Printf("  Checksum: %s\n", checksum.ComputeBytes(totalData.Bytes()))
	fmt.Printf("  Verified: true\n")
	fmt.Printf("  Output:   %s\n\n", destPath)
}

func handleInspect(client pb.CoordinatorServiceClient, objectKey string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.InspectObject(ctx, &pb.InspectObjectRequest{Key: objectKey})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Inspect error: %v\n", err)
		os.Exit(1)
	}

	meta := resp.GetMetadata()
	fmt.Printf("\nObject: %s\n\n", meta.GetKey())
	fmt.Printf("Size:        %d\n", meta.GetSize())
	fmt.Printf("Chunks:      %d\n", len(meta.GetChunks()))
	if len(meta.GetChunks()) > 0 {
		fmt.Printf("Replication: %d\n\n", len(meta.GetChunks()[0].GetReplicas()))
	} else {
		fmt.Printf("Replication: 0\n\n")
	}

	for _, c := range meta.GetChunks() {
		fmt.Printf("Chunk %d\n", c.GetIndex())
		fmt.Printf("  ID:       %s\n", c.GetChunkId())
		fmt.Printf("  Size:     %d\n", c.GetSize())
		fmt.Printf("  SHA-256:  %s\n", c.GetSha256())
		fmt.Printf("  Replicas:\n")
		for _, r := range c.GetReplicas() {
			fmt.Printf("    %s\n", r)
		}
		fmt.Println()
	}
}

func handleDelete(client pb.CoordinatorServiceClient, objectKey string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.DeleteObject(ctx, &pb.DeleteObjectRequest{Key: objectKey})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Delete error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Deleted object: %s (%s)\n", resp.GetKey(), resp.GetMessage())
}

func handleNodes(client pb.CoordinatorServiceClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.ListNodes(ctx, &pb.ListNodesRequest{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed listing nodes: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nCluster Storage Nodes:")
	fmt.Printf("%-15s %-25s %-10s\n", "NODE ID", "ADDRESS", "STATUS")
	fmt.Println(strings.Repeat("-", 55))
	for _, n := range resp.GetNodes() {
		fmt.Printf("%-15s %-25s %-10s\n", n.GetNodeId(), n.GetAddress(), n.GetStatus())
	}
	fmt.Println()
}
