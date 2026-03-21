package dae

import (
	"github.com/gophics/ravenporter/internal/mathx"
	"github.com/gophics/ravenporter/ir"
)

func convertDAECameras(cams []xmlCamera, asset *ir.Asset) map[string]int {
	m := make(map[string]int, len(cams))
	for _, c := range cams {
		cam := &ir.Camera{Name: c.Name}
		if cam.Name == "" {
			cam.Name = c.ID
		}
		if p := c.Optic.Profile.Persp; p != nil {
			fov := p.YFOV.Float
			if fov == 0 {
				fov = p.XFOV.Float
			}
			cam.Perspective = &ir.PerspectiveCamera{
				FOV:  float32(fov * mathx.DegToRad),
				Near: float32(p.ZNear.Float),
				Far:  float32(p.ZFar.Float),
			}
		}
		if o := c.Optic.Profile.Ortho; o != nil {
			cam.Orthographic = &ir.OrthographicCamera{
				XMag: float32(o.XMag.Float),
				YMag: float32(o.YMag.Float),
				Near: float32(o.ZNear.Float),
				Far:  float32(o.ZFar.Float),
			}
		}
		m["#"+c.ID] = len(asset.Cameras)
		asset.Cameras = append(asset.Cameras, cam)
	}
	return m
}

func convertDAELights(lts []xmlLight, asset *ir.Asset) map[string]int {
	m := make(map[string]int, len(lts))
	for _, l := range lts {
		light := &ir.Light{
			Name:      l.Name,
			Color:     [3]float32{1, 1, 1},
			Intensity: 1.0,
		}
		if light.Name == "" {
			light.Name = l.ID
		}
		switch {
		case l.Tech.Point != nil:
			light.Point = &ir.PointLight{}
			if l.Tech.Point.Color != "" {
				light.Color = parseColor3(l.Tech.Point.Color)
			}
		case l.Tech.Directional != nil:
			light.Directional = &ir.DirectionalLight{}
			if l.Tech.Directional.Color != "" {
				light.Color = parseColor3(l.Tech.Directional.Color)
			}
		case l.Tech.Spot != nil:
			angle := float32(l.Tech.Spot.FalloffAngle.Float * mathx.DegToRad)
			light.Spot = &ir.SpotLight{
				InnerConeAngle: angle,
				OuterConeAngle: angle,
			}
			if l.Tech.Spot.Color != "" {
				light.Color = parseColor3(l.Tech.Spot.Color)
			}
		default:
			light.Point = &ir.PointLight{}
		}
		m["#"+l.ID] = len(asset.Lights)
		asset.Lights = append(asset.Lights, light)
	}
	return m
}
