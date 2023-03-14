package slowjoe

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestProxyCreation(t *testing.T) {
	Convey("Given default configuration", t, func() {

		version := "1.2.3"
		bind := "localhost:1234"
		upstream := "127.0.0.1:7890"
		Convey("When proxy instance is created", func() {

			proxy, err := New(
				Version(version),
				Bind(bind),
				Upstream(upstream),
			)

			Convey("Then proxy instance is returned and has version set correctly", func() {
				So(err, ShouldBeNil)
				So(proxy, ShouldNotBeNil)
				So(proxy.version, ShouldEqual, version)
				So(proxy.bind, ShouldEqual, bind)
				So(proxy.upstream, ShouldEqual, upstream)
			})
		})

	})
}
