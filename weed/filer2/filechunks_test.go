package filer2

import (
	"log"
	"testing"

	"github.com/chrislusf/seaweedfs/weed/pb/filer_pb"
)

func TestCompactFileChunks(t *testing.T) {
	chunks := []*filer_pb.FileChunk{
		{Offset: 10, Size: 100, FileId: "abc", Mtime: 50},
		{Offset: 100, Size: 100, FileId: "def", Mtime: 100},
		{Offset: 200, Size: 100, FileId: "ghi", Mtime: 200},
		{Offset: 110, Size: 200, FileId: "jkl", Mtime: 300},
	}

	compacted, garbarge := CompactFileChunks(chunks)

	log.Printf("Compacted: %+v", compacted)
	log.Printf("Garbage  : %+v", garbarge)

	if len(compacted) != 3 {
		t.Fatalf("unexpected compacted: %d", len(compacted))
	}
	if len(garbarge) != 1 {
		t.Fatalf("unexpected garbarge: %d", len(garbarge))
	}

}

func TestIntervalMerging(t *testing.T) {

	testcases := []struct {
		Chunks   []*filer_pb.FileChunk
		Expected []*visibleInterval
	}{
		// case 0: normal
		{
			Chunks: []*filer_pb.FileChunk{
				{Offset: 0, Size: 100, FileId: "abc", Mtime: 123},
				{Offset: 100, Size: 100, FileId: "asdf", Mtime: 134},
				{Offset: 200, Size: 100, FileId: "fsad", Mtime: 353},
			},
			Expected: []*visibleInterval{
				{start: 0, stop: 100, fileId: "abc"},
				{start: 100, stop: 200, fileId: "asdf"},
				{start: 200, stop: 300, fileId: "fsad"},
			},
		},
		// case 1: updates overwrite full chunks
		{
			Chunks: []*filer_pb.FileChunk{
				{Offset: 0, Size: 100, FileId: "abc", Mtime: 123},
				{Offset: 0, Size: 200, FileId: "asdf", Mtime: 134},
			},
			Expected: []*visibleInterval{
				{start: 0, stop: 200, fileId: "asdf"},
			},
		},
		// case 2: updates overwrite part of previous chunks
		{
			Chunks: []*filer_pb.FileChunk{
				{Offset: 0, Size: 100, FileId: "abc", Mtime: 123},
				{Offset: 0, Size: 50, FileId: "asdf", Mtime: 134},
			},
			Expected: []*visibleInterval{
				{start: 0, stop: 50, fileId: "asdf"},
				{start: 50, stop: 100, fileId: "abc"},
			},
		},
		// case 3: updates overwrite full chunks
		{
			Chunks: []*filer_pb.FileChunk{
				{Offset: 0, Size: 100, FileId: "abc", Mtime: 123},
				{Offset: 0, Size: 200, FileId: "asdf", Mtime: 134},
				{Offset: 50, Size: 250, FileId: "xxxx", Mtime: 154},
			},
			Expected: []*visibleInterval{
				{start: 0, stop: 50, fileId: "asdf"},
				{start: 50, stop: 300, fileId: "xxxx"},
			},
		},
		// case 4: updates far away from prev chunks
		{
			Chunks: []*filer_pb.FileChunk{
				{Offset: 0, Size: 100, FileId: "abc", Mtime: 123},
				{Offset: 0, Size: 200, FileId: "asdf", Mtime: 134},
				{Offset: 250, Size: 250, FileId: "xxxx", Mtime: 154},
			},
			Expected: []*visibleInterval{
				{start: 0, stop: 200, fileId: "asdf"},
				{start: 250, stop: 500, fileId: "xxxx"},
			},
		},
		// case 5: updates overwrite full chunks
		{
			Chunks: []*filer_pb.FileChunk{
				{Offset: 0, Size: 100, FileId: "abc", Mtime: 123},
				{Offset: 0, Size: 200, FileId: "asdf", Mtime: 184},
				{Offset: 70, Size: 150, FileId: "abc", Mtime: 143},
				{Offset: 80, Size: 100, FileId: "xxxx", Mtime: 134},
			},
			Expected: []*visibleInterval{
				{start: 0, stop: 200, fileId: "asdf"},
				{start: 200, stop: 220, fileId: "abc"},
			},
		},
		// case 6: same updates
		{
			Chunks: []*filer_pb.FileChunk{
				{Offset: 0, Size: 100, FileId: "abc", Mtime: 123},
				{Offset: 0, Size: 100, FileId: "abc", Mtime: 123},
				{Offset: 0, Size: 100, FileId: "abc", Mtime: 123},
			},
			Expected: []*visibleInterval{
				{start: 0, stop: 100, fileId: "abc"},
			},
		},
	}

	for i, testcase := range testcases {
		log.Printf("++++++++++ merged test case %d ++++++++++++++++++++", i)
		intervals := nonOverlappingVisibleIntervals(testcase.Chunks)
		for x, interval := range intervals {
			log.Printf("test case %d, interval %d, start=%d, stop=%d, fileId=%s",
				i, x, interval.start, interval.stop, interval.fileId)
			if interval.start != testcase.Expected[x].start {
				t.Fatalf("failed on test case %d, interval %d, start %d, expect %d",
					i, x, interval.start, testcase.Expected[x].start)
			}
			if interval.stop != testcase.Expected[x].stop {
				t.Fatalf("failed on test case %d, interval %d, stop %d, expect %d",
					i, x, interval.stop, testcase.Expected[x].stop)
			}
			if interval.fileId != testcase.Expected[x].fileId {
				t.Fatalf("failed on test case %d, interval %d, chunkId %s, expect %s",
					i, x, interval.fileId, testcase.Expected[x].fileId)
			}
		}
		if len(intervals) != len(testcase.Expected) {
			t.Fatalf("failed to compact test case %d, len %d expected %d", i, len(intervals), len(testcase.Expected))
		}
	}

}

func TestChunksReading(t *testing.T) {

	testcases := []struct {
		Chunks   []*filer_pb.FileChunk
		Offset   int64
		Size     int
		Expected []*ChunkView
	}{
		// case 0: normal
		{
			Chunks: []*filer_pb.FileChunk{
				{Offset: 0, Size: 100, FileId: "abc", Mtime: 123},
				{Offset: 100, Size: 100, FileId: "asdf", Mtime: 134},
				{Offset: 200, Size: 100, FileId: "fsad", Mtime: 353},
			},
			Offset: 0,
			Size:   250,
			Expected: []*ChunkView{
				{Offset: 0, Size: 100, FileId: "abc", LogicOffset: 0},
				{Offset: 0, Size: 100, FileId: "asdf", LogicOffset: 100},
				{Offset: 0, Size: 50, FileId: "fsad", LogicOffset: 200},
			},
		},
		// case 1: updates overwrite full chunks
		{
			Chunks: []*filer_pb.FileChunk{
				{Offset: 0, Size: 100, FileId: "abc", Mtime: 123},
				{Offset: 0, Size: 200, FileId: "asdf", Mtime: 134},
			},
			Offset: 50,
			Size:   100,
			Expected: []*ChunkView{
				{Offset: 50, Size: 100, FileId: "asdf", LogicOffset: 50},
			},
		},
		// case 2: updates overwrite part of previous chunks
		{
			Chunks: []*filer_pb.FileChunk{
				{Offset: 0, Size: 100, FileId: "abc", Mtime: 123},
				{Offset: 0, Size: 50, FileId: "asdf", Mtime: 134},
			},
			Offset: 25,
			Size:   50,
			Expected: []*ChunkView{
				{Offset: 25, Size: 25, FileId: "asdf", LogicOffset: 25},
				{Offset: 0, Size: 25, FileId: "abc", LogicOffset: 50},
			},
		},
		// case 3: updates overwrite full chunks
		{
			Chunks: []*filer_pb.FileChunk{
				{Offset: 0, Size: 100, FileId: "abc", Mtime: 123},
				{Offset: 0, Size: 200, FileId: "asdf", Mtime: 134},
				{Offset: 50, Size: 250, FileId: "xxxx", Mtime: 154},
			},
			Offset: 0,
			Size:   200,
			Expected: []*ChunkView{
				{Offset: 0, Size: 50, FileId: "asdf", LogicOffset: 0},
				{Offset: 0, Size: 150, FileId: "xxxx", LogicOffset: 50},
			},
		},
		// case 4: updates far away from prev chunks
		{
			Chunks: []*filer_pb.FileChunk{
				{Offset: 0, Size: 100, FileId: "abc", Mtime: 123},
				{Offset: 0, Size: 200, FileId: "asdf", Mtime: 134},
				{Offset: 250, Size: 250, FileId: "xxxx", Mtime: 154},
			},
			Offset: 0,
			Size:   400,
			Expected: []*ChunkView{
				{Offset: 0, Size: 200, FileId: "asdf", LogicOffset: 0},
				// {Offset: 0, Size: 150, FileId: "xxxx"}, // missing intervals should not happen
			},
		},
		// case 5: updates overwrite full chunks
		{
			Chunks: []*filer_pb.FileChunk{
				{Offset: 0, Size: 100, FileId: "abc", Mtime: 123},
				{Offset: 0, Size: 200, FileId: "asdf", Mtime: 184},
				{Offset: 70, Size: 150, FileId: "abc", Mtime: 143},
				{Offset: 80, Size: 100, FileId: "xxxx", Mtime: 134},
			},
			Offset: 0,
			Size:   220,
			Expected: []*ChunkView{
				{Offset: 0, Size: 200, FileId: "asdf", LogicOffset: 0},
				{Offset: 0, Size: 20, FileId: "abc", LogicOffset: 200},
			},
		},
		// case 6: same updates
		{
			Chunks: []*filer_pb.FileChunk{
				{Offset: 0, Size: 100, FileId: "abc", Mtime: 123},
				{Offset: 0, Size: 100, FileId: "abc", Mtime: 123},
				{Offset: 0, Size: 100, FileId: "abc", Mtime: 123},
			},
			Offset: 0,
			Size:   100,
			Expected: []*ChunkView{
				{Offset: 0, Size: 100, FileId: "abc", LogicOffset: 0},
			},
		},
		// case 7: edge cases
		{
			Chunks: []*filer_pb.FileChunk{
				{Offset: 0, Size: 100, FileId: "abc", Mtime: 123},
				{Offset: 100, Size: 100, FileId: "asdf", Mtime: 134},
				{Offset: 200, Size: 100, FileId: "fsad", Mtime: 353},
			},
			Offset: 0,
			Size:   200,
			Expected: []*ChunkView{
				{Offset: 0, Size: 100, FileId: "abc", LogicOffset: 0},
				{Offset: 0, Size: 100, FileId: "asdf", LogicOffset: 100},
			},
		},
	}

	for i, testcase := range testcases {
		log.Printf("++++++++++ read test case %d ++++++++++++++++++++", i)
		chunks := ViewFromChunks(testcase.Chunks, testcase.Offset, testcase.Size)
		for x, chunk := range chunks {
			log.Printf("read case %d, chunk %d, offset=%d, size=%d, fileId=%s",
				i, x, chunk.Offset, chunk.Size, chunk.FileId)
			if chunk.Offset != testcase.Expected[x].Offset {
				t.Fatalf("failed on read case %d, chunk %d, Offset %d, expect %d",
					i, x, chunk.Offset, testcase.Expected[x].Offset)
			}
			if chunk.Size != testcase.Expected[x].Size {
				t.Fatalf("failed on read case %d, chunk %d, Size %d, expect %d",
					i, x, chunk.Size, testcase.Expected[x].Size)
			}
			if chunk.FileId != testcase.Expected[x].FileId {
				t.Fatalf("failed on read case %d, chunk %d, FileId %s, expect %s",
					i, x, chunk.FileId, testcase.Expected[x].FileId)
			}
			if chunk.LogicOffset != testcase.Expected[x].LogicOffset {
				t.Fatalf("failed on read case %d, chunk %d, LogicOffset %d, expect %d",
					i, x, chunk.LogicOffset, testcase.Expected[x].LogicOffset)
			}
		}
		if len(chunks) != len(testcase.Expected) {
			t.Fatalf("failed to read test case %d, len %d expected %d", i, len(chunks), len(testcase.Expected))
		}
	}

}