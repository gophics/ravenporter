package mathx

var (
	IdentityQuat  = [4]float32{0, 0, 0, 1} //nolint:gochecknoglobals // identity rotation
	IdentityScale = [3]float32{1, 1, 1}    //nolint:gochecknoglobals // identity scale
	IdentityMat4  = [16]float32{           //nolint:gochecknoglobals // identity matrix
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
)
