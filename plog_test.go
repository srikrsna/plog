package plog_test

import (
	"io"
	"testing"

	"github.com/srikrsna/plog"
	"golang.org/x/exp/slog"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func BenchmarkMsg(b *testing.B) {
	l := slog.New(slog.HandlerOptions{
		Level: slog.ErrorLevel,
	}.NewTextHandler(io.Discard))
	payload := &timestamppb.Timestamp{}
	b.ReportAllocs()
	b.Run("without-logger", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = plog.Msg("msg", payload)
		}
	})
	b.Run("enabled", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			l.LogAttrs(slog.ErrorLevel, "error", plog.Msg("msg", payload))
		}
	})
	b.Run("disabled", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			l.LogAttrs(slog.InfoLevel, "error", plog.Msg("msg", payload))
		}
	})
}
