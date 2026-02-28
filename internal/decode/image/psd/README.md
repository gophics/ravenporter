# PSD Decoder

Adobe Photoshop Document parser with composite pixel decode.

## Extensions

`.psd`, `.psb`

## Supported Features

| Feature | Status |
| :--- | :---: |
| File Header Parsing | ✅ |
| Dimensions, Channels, Depth, Mode | ✅ |
| Layer & Mask Info (Layer Count) | ✅ |
| Composite Pixel Decode (Raw) | ✅ |
| Composite Pixel Decode (PackBits RLE) | ✅ |
| Composite Pixel Decode (ZIP) | ✅ |
| Composite Pixel Decode (ZIP with Prediction) | ✅ |
| 8-bit Depth | ✅ |
| 16-bit Depth (normalized to 8-bit) | ✅ |
| 32-bit Depth / HDR Float (tone-mapped to 8-bit) | ✅ |
| RGB Mode | ✅ |
| Grayscale Mode (→ RGBA) | ✅ |
| CMYK Mode (→ RGBA) | ✅ |
| Metadata: BitDepth, ColorMode, LayerCount | ✅ |
