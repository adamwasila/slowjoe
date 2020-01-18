package slowjoe

import (
	"testing"

	"github.com/adamwasila/slowjoe/config"
	"github.com/sirupsen/logrus"
	. "github.com/smartystreets/goconvey/convey"
)

func TestProxyCreation(t *testing.T) {
	presetup()

	Convey("Given default configuration", t, func() {

		version := "1.2.3"
		config := config.Config{}
		shutdowner := SignalShutdowner{}

		Convey("When proxy instance is created", func() {

			proxy := New(version, config, &shutdowner)

			Convey("Then proxy instance is returned and has version set correctly", func() {
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
