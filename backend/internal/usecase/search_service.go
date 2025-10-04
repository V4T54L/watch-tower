package usecase

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"

	"github.com/V4T54L/watch-tower/internal/domain"
	"github.com/klauspost/compress/zstd"
	"go.opentelemetry.io/otel"
)

type searchService struct {
	logRepo     domain.LogRepository
	s3ChunkRepo domain.S3ChunkRepository
	s3Client    domain.Client
}

func NewSearchService(logRepo domain.LogRepository, s3ChunkRepo domain.S3ChunkRepository, s3Client domain.Client) SearchUseCase {
	return &searchService{
		logRepo:     logRepo,
		s3ChunkRepo: s3ChunkRepo,
		s3Client:    s3Client,
	}
}

func (s *searchService) Search(ctx context.Context, params SearchParams) (*SearchResult, error) {
	_, span := otel.Tracer("search-service").Start(ctx, "Search")
	defer span.End()

	var wg sync.WaitGroup
	hotLogsChan := make(chan []*domain.Log, 1)
	coldLogsChan := make(chan []*domain.Log, 1)
	errChan := make(chan error, 2)

	// Search hot storage
	wg.Add(1)
	go func() {
		defer wg.Done()
		logs, _, err := s.logRepo.Search(ctx, params.TenantID, params.Query, params.Start, params.End, params.Limit, params.Cursor)
		if err != nil {
			errChan <- err
			return
		}
		hotLogsChan <- logs
	}()

	// Search cold storage
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Only search cold if S3 is configured
		if s.s3Client != nil && s.s3ChunkRepo != nil {
			logs, err := s.searchColdStorage(ctx, params)
			if err != nil {
				errChan <- err
				return
			}
			coldLogsChan <- logs
		} else {
			coldLogsChan <- []*domain.Log{}
		}
	}()

	wg.Wait()
	close(hotLogsChan)
	close(coldLogsChan)
	close(errChan)

	for err := range errChan {
		return nil, err // Return on first error
	}

	hotLogs := <-hotLogsChan
	coldLogs := <-coldLogsChan

	allLogs := append(hotLogs, coldLogs...)

	// Sort by timestamp descending
	sort.Slice(allLogs, func(i, j int) bool {
		return allLogs[i].Timestamp.After(allLogs[j].Timestamp)
	})

	// Apply limit
	if len(allLogs) > params.Limit {
		allLogs = allLogs[:params.Limit]
	}

	return &SearchResult{Logs: allLogs}, nil
}

func (s *searchService) searchColdStorage(ctx context.Context, params SearchParams) ([]*domain.Log, error) {
	_, span := otel.Tracer("search-service").Start(ctx, "searchColdStorage")
	defer span.End()

	chunks, err := s.s3ChunkRepo.FindChunksByTimeRange(ctx, params.TenantID, params.Start, params.End)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var matchingLogs []*domain.Log

	for _, chunk := range chunks {
		wg.Add(1)
		go func(c *domain.S3ChunkMetadata) {
			defer wg.Done()
			compressedData, err := s.s3Client.Download(ctx, c.S3Key)
			if err != nil {
				// Log error but continue
				return
			}

			reader, err := zstd.NewReader(bytes.NewReader(compressedData))
			if err != nil {
				return
			}
			defer reader.Close()

			scanner := bufio.NewScanner(reader)
			for scanner.Scan() {
				var log domain.Log
				if err := json.Unmarshal(scanner.Bytes(), &log); err == nil {
					if log.Timestamp.After(params.Start) && log.Timestamp.Before(params.End) {
						if params.Query == "" || strings.Contains(strings.ToLower(log.Message), strings.ToLower(params.Query)) {
							mu.Lock()
							matchingLogs = append(matchingLogs, &log)
							mu.Unlock()
						}
					}
				}
			}
		}(chunk)
	}

	wg.Wait()
	return matchingLogs, nil
}
