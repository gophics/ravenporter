package dae

import "encoding/xml"

type collada struct {
	XMLName xml.Name `xml:"COLLADA"`
	Version string   `xml:"version,attr"`
	Asset   asset    `xml:"asset"`

	LibGeometries   libGeometries   `xml:"library_geometries"`
	LibMaterials    libMaterials    `xml:"library_materials"`
	LibEffects      libEffects      `xml:"library_effects"`
	LibImages       libImages       `xml:"library_images"`
	LibVisualScenes libVisualScenes `xml:"library_visual_scenes"`
	LibAnimations   libAnimations   `xml:"library_animations"`
	LibControllers  libControllers  `xml:"library_controllers"`
	LibCameras      libCameras      `xml:"library_cameras"`
	LibLights       libLights       `xml:"library_lights"`
}

type asset struct {
	UpAxis string `xml:"up_axis"`
	Unit   unit   `xml:"unit"`
}

type unit struct {
	Meter float64 `xml:"meter,attr"`
	Name  string  `xml:"name,attr"`
}

type libGeometries struct {
	Geometries []geometry `xml:"geometry"`
}

type geometry struct {
	ID   string  `xml:"id,attr"`
	Name string  `xml:"name,attr"`
	Mesh xmlMesh `xml:"mesh"`
}

type xmlMesh struct {
	Sources    []source      `xml:"source"`
	Vertices   vertices      `xml:"vertices"`
	Triangles  []xmlTris     `xml:"triangles"`
	Polylist   []polylist    `xml:"polylist"`
	Lines      []xmlTris     `xml:"lines"`
	Tristrips  []xmlTris     `xml:"tristrips"`
	Trifans    []xmlTris     `xml:"trifans"`
	Linestrips []xmlTris     `xml:"linestrips"`
	Polygons   []xmlPolygons `xml:"polygons"`
}

type source struct {
	ID         string     `xml:"id,attr"`
	FloatArray floatArray `xml:"float_array"`
}

type floatArray struct {
	Count int    `xml:"count,attr"`
	Data  string `xml:",chardata"`
}

type vertices struct {
	ID     string  `xml:"id,attr"`
	Inputs []input `xml:"input"`
}

type input struct {
	Semantic string `xml:"semantic,attr"`
	Source   string `xml:"source,attr"`
	Offset   int    `xml:"offset,attr"`
	Set      string `xml:"set,attr"`
}

type xmlTris struct {
	Count    int     `xml:"count,attr"`
	Material string  `xml:"material,attr"`
	Inputs   []input `xml:"input"`
	P        string  `xml:"p"`
}

type polylist struct {
	Count    int     `xml:"count,attr"`
	Material string  `xml:"material,attr"`
	Inputs   []input `xml:"input"`
	VCount   string  `xml:"vcount"`
	P        string  `xml:"p"`
}

type xmlPolygons struct {
	Count    int      `xml:"count,attr"`
	Material string   `xml:"material,attr"`
	Inputs   []input  `xml:"input"`
	Ps       []string `xml:"p"`
}

type libMaterials struct {
	Materials []xmlMaterial `xml:"material"`
}

type xmlMaterial struct {
	ID   string     `xml:"id,attr"`
	Name string     `xml:"name,attr"`
	Inst instEffect `xml:"instance_effect"`
}

type instEffect struct {
	URL string `xml:"url,attr"`
}

type libEffects struct {
	Effects []effect `xml:"effect"`
}

type effect struct {
	ID       string     `xml:"id,attr"`
	Profiles []profComm `xml:"profile_COMMON"`
}

type profComm struct {
	Technique technique `xml:"technique"`
}

type technique struct {
	Phong   *shading `xml:"phong"`
	Lambert *shading `xml:"lambert"`
	Blinn   *shading `xml:"blinn"`
}

type shading struct {
	Ambient   colorOrTexture `xml:"ambient"`
	Diffuse   colorOrTexture `xml:"diffuse"`
	Specular  colorOrTexture `xml:"specular"`
	Emission  colorOrTexture `xml:"emission"`
	Shininess floatVal       `xml:"shininess"`
}

type colorOrTexture struct {
	Color   string    `xml:"color"`
	Texture xmlTexRef `xml:"texture"`
}

type xmlTexRef struct {
	Texture  string `xml:"texture,attr"`
	TexCoord string `xml:"texcoord,attr"`
}

type floatVal struct {
	Float float64 `xml:"float"`
}

type libImages struct {
	Images []xmlImage `xml:"image"`
}

type xmlImage struct {
	ID   string   `xml:"id,attr"`
	Name string   `xml:"name,attr"`
	Init initFrom `xml:"init_from"`
}

type initFrom struct {
	Path string `xml:",chardata"`
}

type libVisualScenes struct {
	Scenes []visualScene `xml:"visual_scene"`
}

type visualScene struct {
	ID    string    `xml:"id,attr"`
	Name  string    `xml:"name,attr"`
	Nodes []xmlNode `xml:"node"`
}

type xmlNode struct {
	ID       string    `xml:"id,attr"`
	Name     string    `xml:"name,attr"`
	Type     string    `xml:"type,attr"`
	Matrix   string    `xml:"matrix"`
	InstGeom []instGeo `xml:"instance_geometry"`
	InstCam  []instRef `xml:"instance_camera"`
	InstLit  []instRef `xml:"instance_light"`
	Children []xmlNode `xml:"node"`
}

type instRef struct {
	URL string `xml:"url,attr"`
}

type instGeo struct {
	URL string `xml:"url,attr"`
}

type libAnimations struct {
	Animations []xmlAnimation `xml:"animation"`
}

type xmlAnimation struct {
	ID       string           `xml:"id,attr"`
	Name     string           `xml:"name,attr"`
	Sources  []source         `xml:"source"`
	Samplers []xmlAnimSampler `xml:"sampler"`
	Channels []xmlAnimChannel `xml:"channel"`
	Children []xmlAnimation   `xml:"animation"`
}

type xmlAnimSampler struct {
	ID     string  `xml:"id,attr"`
	Inputs []input `xml:"input"`
}

type xmlAnimChannel struct {
	Source string `xml:"source,attr"`
	Target string `xml:"target,attr"`
}

type libControllers struct {
	Controllers []xmlController `xml:"controller"`
}

type xmlController struct {
	ID    string   `xml:"id,attr"`
	Name  string   `xml:"name,attr"`
	Skin  xmlSkin  `xml:"skin"`
	Morph xmlMorph `xml:"morph"`
}

type xmlMorph struct {
	Source  string          `xml:"source,attr"`
	Method  string          `xml:"method,attr"`
	Sources []source        `xml:"source"`
	Targets xmlMorphTargets `xml:"targets"`
}

type xmlMorphTargets struct {
	Inputs []input `xml:"input"`
}

type xmlSkin struct {
	Source        string           `xml:"source,attr"`
	Sources       []source         `xml:"source"`
	Joints        xmlJoints        `xml:"joints"`
	VertexWeights xmlVertexWeights `xml:"vertex_weights"`
}

type xmlJoints struct {
	Inputs []input `xml:"input"`
}

type xmlVertexWeights struct {
	Count  int     `xml:"count,attr"`
	Inputs []input `xml:"input"`
	VCount string  `xml:"vcount"`
	V      string  `xml:"v"`
}

type libCameras struct {
	Cameras []xmlCamera `xml:"camera"`
}

type xmlCamera struct {
	ID    string   `xml:"id,attr"`
	Name  string   `xml:"name,attr"`
	Optic xmlOptic `xml:"optics"`
}

type xmlOptic struct {
	Profile xmlOpticProfile `xml:"technique_common"`
}

type xmlOpticProfile struct {
	Persp *xmlPersp `xml:"perspective"`
	Ortho *xmlOrtho `xml:"orthographic"`
}

type xmlPersp struct {
	XFOV  floatVal `xml:"xfov"`
	YFOV  floatVal `xml:"yfov"`
	ZNear floatVal `xml:"znear"`
	ZFar  floatVal `xml:"zfar"`
}

type xmlOrtho struct {
	XMag  floatVal `xml:"xmag"`
	YMag  floatVal `xml:"ymag"`
	ZNear floatVal `xml:"znear"`
	ZFar  floatVal `xml:"zfar"`
}

type libLights struct {
	Lights []xmlLight `xml:"light"`
}

type xmlLight struct {
	ID   string       `xml:"id,attr"`
	Name string       `xml:"name,attr"`
	Tech xmlLightTech `xml:"technique_common"`
}

type xmlLightTech struct {
	Point       *xmlLightData `xml:"point"`
	Directional *xmlLightData `xml:"directional"`
	Spot        *xmlLightSpot `xml:"spot"`
}

type xmlLightData struct {
	Color string `xml:"color"`
}

type xmlLightSpot struct {
	Color        string   `xml:"color"`
	FalloffAngle floatVal `xml:"falloff_angle"`
}
