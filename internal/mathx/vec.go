// Package mathx provides the math facade for RavenPorter.
// All math types and operations are re-exported from go-gl/mathgl/mgl32.
// The rest of the codebase imports only this package, never mgl32 directly.
package mathx

import "github.com/go-gl/mathgl/mgl32"

type Vec3 = mgl32.Vec3
type Vec4 = mgl32.Vec4
type Mat3 = mgl32.Mat3
type Mat4 = mgl32.Mat4
type Quat = mgl32.Quat

var Ident4 = mgl32.Ident4
