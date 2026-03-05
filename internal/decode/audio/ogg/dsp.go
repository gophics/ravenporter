package ogg

const (
	floorRangeCount = 4
	floorNormDiv    = 255.0
	residuePasses   = 8
)

var floorRangeVals = [floorRangeCount]int{256, 128, 86, 64}

func decodeFloor1(br *bitReader, setup *vorbisSetup, floor *vorbisFloor, channelData []float32, blockSize int) bool {
	if br.readBit() == 0 {
		return false
	}

	amp := br.readBits(ilog(floorRangeVals[floor.multiplier-1] - 1))

	for i := range floor.partitions {
		class := floor.partitionClass[i]
		cdim := floor.classDimensions[class]
		csub := floor.classSubclasses[class]

		if csub != 0 {
			cb := &setup.codebooks[floor.classMasterbooks[class]]
			if _, err := readHuffman(br, cb.huffTree); err != nil {
				return false
			}
		}

		for j := range cdim {
			_ = j
			if csub != 0 {
				br.readBits(1)
			}
		}
	}

	multiplier := float32(amp) / floorNormDiv
	for i := range blockSize / windowHalf {
		channelData[i] *= multiplier
	}

	return true
}

func decodeResidue(br *bitReader, fd *frameDecoder, residue *vorbisResidue, chToDecode []int) {
	setup := fd.setup

	begin := residue.begin
	end := residue.end
	if end > fd.prevSize/windowHalf {
		end = fd.prevSize / windowHalf
	}
	partitionSize := residue.partitionSize
	classbook := residue.classbook

	classwordsPerCodeword := setup.codebooks[classbook].dimensions
	nToRead := end - begin
	partitionsToRead := nToRead / partitionSize

	if partitionsToRead == 0 {
		return
	}

	for pass := range residuePasses {
		partitionCount := 0
		fd.vqClasses = fd.vqClasses[:0]

		for partitionCount < partitionsToRead {
			if pass == 0 {
				cb := &setup.codebooks[classbook]
				entry, err := readHuffman(br, cb.huffTree)
				if err != nil {
					return
				}
				fd.vqClasses = append(fd.vqClasses, entry)
			}

			for i := 0; i < classwordsPerCodeword && partitionCount < partitionsToRead; i++ {
				vqClass := 0
				if pass == 0 && len(fd.vqClasses) > 0 {
					vqClass = fd.vqClasses[len(fd.vqClasses)-1]
				}

				vqBookID := residue.books[vqClass%residue.classifications][pass]
				if vqBookID >= 0 {
					cb := &setup.codebooks[vqBookID]
					entry, err := readHuffman(br, cb.huffTree)
					if err == nil {
						applyResidueVQ(fd, residue, cb, entry, chToDecode, begin, partitionCount, partitionSize, i)
					}
				}
				partitionCount++
			}
		}
	}
}

//nolint:cyclop
func applyResidueVQ(
	fd *frameDecoder, residue *vorbisResidue, cb *vorbisCodebook,
	entry int, chToDecode []int, begin, partitionCount, partitionSize, i int,
) {
	chCount := len(chToDecode)

	switch residue.residueType {
	case 0:
		for k := range cb.dimensions {
			for l := range chToDecode {
				ch := chToDecode[l]
				idx := begin + partitionCount*partitionSize + i*cb.dimensions + k
				if idx < len(fd.channelData[ch]) {
					fd.channelData[ch][idx] += cb.lookupVals[entry*cb.dimensions+k]
				}
			}
		}
	case 1:
		for k := range cb.dimensions {
			chOffset := (i*cb.dimensions + k) % chCount
			ch := chToDecode[chOffset]
			idx := begin + partitionCount*partitionSize + (i*cb.dimensions+k)/chCount
			if idx < len(fd.channelData[ch]) {
				fd.channelData[ch][idx] += cb.lookupVals[entry*cb.dimensions+k]
			}
		}
	default:
		for k := range cb.dimensions {
			chOffset := (partitionCount*partitionSize + i*cb.dimensions + k) % chCount
			ch := chToDecode[chOffset]
			idx := begin + (partitionCount*partitionSize+i*cb.dimensions+k)/chCount
			if idx < len(fd.channelData[ch]) {
				fd.channelData[ch][idx] += cb.lookupVals[entry*cb.dimensions+k]
			}
		}
	}
}
