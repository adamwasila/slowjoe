package slowjoe

import (
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRandomTName(t *testing.T) {
	Convey("Given random t-name", t, func() {
		name := randomT()
		names := strings.Split(name, "-")
		Convey("Name should follow pattern: T.*-T.*", func() {
			So(names, ShouldHaveLength, 2)
			So(names[0], ShouldStartWith, "T")
			So(names[1], ShouldStartWith, "T")
		})
	})
}

func TestRandomRName(t *testing.T) {
	Convey("Given random r-name", t, func() {
		name := randomR()
		names := strings.Split(name, "-")
		Convey("Name should follow pattern: R.*-R.*", func() {
			So(names, ShouldHaveLength, 2)
			So(names[0], ShouldStartWith, "R")
			So(names[1], ShouldStartWith, "R")
		})
	})
}

func TestRandomCName(t *testing.T) {
	Convey("Given random c-name", t, func() {
		name := randomC()
		names := strings.Split(name, "-")
		Convey("Name should follow pattern: C.*-C.*", func() {
			So(names, ShouldHaveLength, 2)
			So(names[0], ShouldStartWith, "C")
			So(names[1], ShouldStartWith, "C")
		})
	})
}
