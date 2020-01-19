package slowjoe

import (
	"testing"

	"github.com/sirupsen/logrus"
	. "github.com/smartystreets/goconvey/convey"
)

func TestProxyCreation(t *testing.T) {
	presetup()

	Convey("Given default configuration", t, func() {

		version := "1.2.3"
		Convey("When proxy instance is created", func() {

			proxy, err := New(
				Version(version),
			)

			Convey("Then proxy instance is returned and has version set correctly", func() {
				So(err, ShouldBeNil)
				So(proxy, ShouldNotBeNil)
				So(proxy.version, ShouldEqual, version)
			})
		})

	})
}

func presetup() {
	//important! this setup may not fail
	logrus.StandardLogger().ExitFunc = func(int) {}
}
