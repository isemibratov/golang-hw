package hw10programoptimization

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"
)

func BenchmarkGetDomainStat(b *testing.B) {
	archive, err := zip.OpenReader("testdata/users.dat.zip")
	if err != nil {
		b.Fatal(err)
	}
	defer archive.Close()

	if len(archive.File) != 1 {
		b.Fatalf("expected one file in the archive, got %d", len(archive.File))
	}
	file, err := archive.File[0].Open()
	if err != nil {
		b.Fatal(err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stat, err := GetDomainStat(bytes.NewReader(data), "biz")
		if err != nil {
			b.Fatal(err)
		}
		if len(stat) == 0 {
			b.Fatal("expected nonempty domain statistics")
		}
	}
}
