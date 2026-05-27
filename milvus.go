package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"google.golang.org/grpc"
)

// MilvusClient manages interactions with Milvus.
type MilvusClient struct {
	client client.Client
	config *Config
}

// NewMilvusClient connects to Milvus, routing through SSH if an SSH client is provided.
func NewMilvusClient(ctx context.Context, cfg *Config, sshClient *SSHClient) (*MilvusClient, error) {
	if cfg.MilvusAddr == "" {
		return nil, errors.New("Milvus address is empty")
	}

	// Strip http:// or https:// prefix if present to prevent gRPC connection issues
	addr := cfg.MilvusAddr
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimPrefix(addr, "https://")

	clientCfg := client.Config{
		Address:  addr,
		Username: cfg.MilvusUser,
		Password: cfg.MilvusPass,
		DBName:   cfg.MilvusDB,
	}

	// Route traffic through SSH dialer if available
	if sshClient != nil {
		dialer := func(dialCtx context.Context, addr string) (net.Conn, error) {
			return sshClient.DialContext(dialCtx, addr)
		}

		clientCfg.DialOptions = []grpc.DialOption{
			grpc.WithContextDialer(dialer),
			grpc.WithInsecure(), // Disable TLS verification for SSH forwarded traffic
			grpc.WithBlock(),
		}
	}

	// Try to connect with a timeout
	connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cli, err := client.NewClient(connectCtx, clientCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Milvus: %w", err)
	}

	return &MilvusClient{
		client: cli,
		config: cfg,
	}, nil
}

// Close closes the Milvus client connection.
func (m *MilvusClient) Close() error {
	if m.client != nil {
		return m.client.Close()
	}
	return nil
}

// ListDatabases retrieves all databases in the cluster.
func (m *MilvusClient) ListDatabases(ctx context.Context) ([]string, error) {
	dbs, err := m.client.ListDatabases(ctx)
	if err != nil {
		return nil, err
	}

	dbNames := make([]string, len(dbs))
	for i, db := range dbs {
		dbNames[i] = db.Name
	}
	return dbNames, nil
}

// ListCollections retrieves all collection names in the current database.
func (m *MilvusClient) ListCollections(ctx context.Context) ([]string, error) {
	colls, err := m.client.ListCollections(ctx)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(colls))
	for i, col := range colls {
		names[i] = col.Name
	}
	return names, nil
}

// DescribeCollection retrieves the schema and configuration of a collection.
func (m *MilvusClient) DescribeCollection(ctx context.Context, collName string) (*entity.Collection, error) {
	return m.client.DescribeCollection(ctx, collName)
}

// DescribeIndex retrieves information about the indexes created on a collection.
func (m *MilvusClient) DescribeIndex(ctx context.Context, collName string, fieldName string) ([]entity.Index, error) {
	return m.client.DescribeIndex(ctx, collName, fieldName)
}

// Query performs a scalar query using an expression.
func (m *MilvusClient) Query(ctx context.Context, collName string, expr string, outputFields []string) (client.ResultSet, error) {
	// If the expression is a wildcard or empty (and not a count query), auto-generate a match-all filter
	isCountQuery := len(outputFields) == 1 && outputFields[0] == "count(*)"
	if (expr == "" || expr == "*" || strings.ToLower(expr) == "all") && !isCountQuery {
		coll, err := m.DescribeCollection(ctx, collName)
		if err != nil {
			return nil, fmt.Errorf("failed to describe collection for query: %w", err)
		}
		var pkField *entity.Field
		for _, f := range coll.Schema.Fields {
			if f.PrimaryKey {
				pkField = f
				break
			}
		}
		if pkField != nil {
			switch pkField.DataType {
			case entity.FieldTypeInt64, entity.FieldTypeInt32, entity.FieldTypeInt16, entity.FieldTypeInt8:
				expr = fmt.Sprintf("%s >= 0", pkField.Name)
			case entity.FieldTypeVarChar, entity.FieldTypeString:
				expr = fmt.Sprintf("%s != ''", pkField.Name)
			}
		}
	}

	// If output fields are empty, we default to describing the collection to get all scalar fields
	if len(outputFields) == 0 {
		coll, err := m.DescribeCollection(ctx, collName)
		if err != nil {
			return nil, fmt.Errorf("failed to auto-resolve output fields: %w", err)
		}
		for _, f := range coll.Schema.Fields {
			// Skip vector fields by default for normal scalar query display
			if f.DataType != entity.FieldTypeFloatVector && f.DataType != entity.FieldTypeBinaryVector {
				outputFields = append(outputFields, f.Name)
			}
		}
	}

	return m.client.Query(
		ctx,
		collName,
		nil, // partitions (nil for all)
		expr,
		outputFields,
	)
}

// Search performs a vector similarity search.
func (m *MilvusClient) Search(ctx context.Context, collName string, vector []float32, vectorField string, limit int, outputFields []string) ([]client.SearchResult, error) {
	if vectorField == "" {
		// Attempt to auto-detect vector field from schema
		coll, err := m.DescribeCollection(ctx, collName)
		if err != nil {
			return nil, fmt.Errorf("failed to auto-resolve vector field: %w", err)
		}
		for _, f := range coll.Schema.Fields {
			if f.DataType == entity.FieldTypeFloatVector {
				vectorField = f.Name
				break
			}
		}
		if vectorField == "" {
			return nil, errors.New("could not find a float vector field in collection schema")
		}
	}	// Build default search params
	sp, err := entity.NewIndexAUTOINDEXSearchParam(1)
	if err != nil {
		return nil, fmt.Errorf("failed to create search param: %w", err)
	}

	return m.client.Search(
		ctx,
		collName,
		nil, // partitions (nil for all)
		"",  // expression filter (empty for none)
		outputFields,
		[]entity.Vector{entity.FloatVector(vector)},
		vectorField,
		entity.L2, // Default metric type
		limit,
		sp,
	)
}
