package main

import (
	"errors"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/doctolib/MailHog/pkg/config"
	"github.com/doctolib/MailHog/pkg/data"
	"github.com/doctolib/MailHog/pkg/storage"
)

// failingStorage wraps a real Storage and forces DeleteAll to fail, to
// exercise the error path of runCleanOnStart without a real backend.
type failingStorage struct {
	storage.Storage
}

func (f *failingStorage) DeleteAll() error {
	return errors.New("boom")
}

func TestRunCleanOnStart(t *testing.T) {
	Convey("runCleanOnStart", t, func() {
		Convey("deletes all messages and returns nil on success", func() {
			s := storage.CreateInMemory()
			_, err := s.Store(&data.Message{
				ID: data.MessageID("test-message"),
				Content: &data.Content{
					Body: "test",
				},
			})
			So(err, ShouldBeNil)
			So(s.Count(), ShouldEqual, 1)

			conf := &config.Config{Storage: s}
			err = runCleanOnStart(conf)

			So(err, ShouldBeNil)
			So(s.Count(), ShouldEqual, 0)
		})

		Convey("returns the storage error on failure", func() {
			conf := &config.Config{Storage: &failingStorage{}}

			err := runCleanOnStart(conf)

			So(err, ShouldNotBeNil)
		})
	})
}
