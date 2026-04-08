---
title: Profiles and Presets
description: Built-in presets, TOML profiles, and the precedence rules RavenPorter uses when options are layered together.
---

Presets are the quick way to get moving. Profiles are for when you want that behavior to survive across runs as TOML.

## Built-In Presets

Use `WithPreset()` with one of:

- `fast`
- `quality`
- `max-quality`

`BuiltInPresetNames()` returns the supported names programmatically.

## Resolving A Serializable Profile

```go
profile, err := ravenporter.ResolveProfile(
	ravenporter.WithPreset(ravenporter.BuiltInPresetQuality),
	ravenporter.WithGlobalScale(2),
	ravenporter.WithEmbedTextures(),
)
if err != nil {
	log.Fatal(err)
}

fmt.Println(profile.Preset, profile.Process.EnabledSteps[0], *profile.Process.GlobalScale)
```

`ResolveProfile()` is useful when you want to inspect or save the effective serializable configuration produced by an option list.

## Load And Save

```go
profile := ravenporter.Profile{
	Version: ravenporter.ProfileVersion,
	Preset:  ravenporter.BuiltInPresetQuality,
}

if err := ravenporter.SaveProfile("profiles/game.toml", profile); err != nil {
	log.Fatal(err)
}

loaded, err := ravenporter.LoadProfile("profiles/game.toml")
if err != nil {
	log.Fatal(err)
}

_ = loaded
```

## Profile Shape

```toml
version = 1
preset = "quality"

[decode]
max_file_size = 268435456
max_vertices = 2000000
max_image_pixels = 16777216
max_audio_samples = 960000

[process]
enable_steps = ["embed-textures"]
disable_steps = ["decode-pixels"]
smooth_normal_angle = 60
max_bone_weights = 4
max_vertices_per_mesh = 65535
max_bones_per_mesh = 64
max_texture_size = 4096
atlas_font_size = 1024
global_scale = 0.01
target_up_axis = "Y"
remove_flags = ["normals", "texcoord0"]
target_sample_rate = 48000
target_channels = 2
degenerate_mode = "convert"
debone_threshold = 0.5
```

## What Can Be Serialized

Profiles can persist:

- `preset`
- decode safeguards such as `max_file_size`, `max_vertices`, `max_image_pixels`, and `max_audio_samples`
- process step toggles through `enable_steps` and `disable_steps`
- dedicated process settings such as `global_scale`, `target_up_axis`, texture-size limits, sample-rate targets, and related tuning

Canonical names are normalized when profiles are parsed or written:

- process-step names use lowercase hyphenated strings such as `embed-textures`
- `remove_flags` uses lowercase names such as `normals`, `texcoord0`, and `weights`
- `target_up_axis` accepts `Y` or `Z`
- `degenerate_mode` accepts `remove` or `convert`

Profiles do not represent every option. For example:

- `WithLoadMask()` is not serializable.
- `WithRegistry()` and `WithLogger()` are runtime concerns, not profile data.

## Precedence Rules

For the library, option order matters. Later options can override earlier ones because options are applied left to right.

```go
ravenporter.ImportPath(
	ctx,
	"scene.fbx",
	ravenporter.WithProfileFile("profiles/game.toml"),
	ravenporter.WithGlobalScale(0.01), // applied after the profile
)
```

The CLI follows the same principle. Flags provided alongside `--profile-file` are assembled afterward, so direct flags win over the loaded profile.

## Process Step Names

The canonical step names used in profiles are the same names returned by the CLI command:

```bash
ravenporter steps
```

See [Custom Detection and Processing](../custom-detection-and-processing/) for more on step overrides and the processing catalog behind them.
