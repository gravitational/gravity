package dns

import "fmt"

// Version is current version of this library.
<<<<<<< HEAD
var Version = V{1, 1, 19}
=======
var Version = V{1, 0, 4}
>>>>>>> 85acc1406... Bump K8s libraries to 1.13.4

// V holds the version of this library.
type V struct {
	Major, Minor, Patch int
}

func (v V) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}
