package cmd

import (
	"archive/zip"
	"errors"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"strings"

	"github.com/Aidoku/aidoku-cli/internal/common"
	"github.com/Aidoku/aidoku-cli/internal/verify"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/xeipuuv/gojsonschema"
)

func verifySchemas(schema gojsonschema.JSONLoader, f *zip.File) error {
	rc, err := f.Open()
	if err != nil {
		color.Red("error: couldn't read %s: %s", f.Name, err)
		return err
	}
	buf := new(strings.Builder)
	io.Copy(buf, rc)
	document := gojsonschema.NewStringLoader(buf.String())
	result, err := gojsonschema.Validate(schema, document)
	if err != nil {
		color.Yellow("warning: could not verify %s: %s", f.Name, err)
		return err
	}
	if !result.Valid() {
		color.Red("error")
		for _, desc := range result.Errors() {
			fmt.Printf("      * %s\n", desc)
		}
		return errors.New("invalid")
	}
	return nil
}

func opaque(im image.Image) bool {
	if oim, ok := im.(interface {
		Opaque() bool
	}); ok {
		return oim.Opaque()
	}

	rect := im.Bounds()
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if _, _, _, a := im.At(x, y).RGBA(); a != 0xffff {
				return false
			}
		}

	}
	return true
}

var verifyCmd = &cobra.Command{
	Use:           "verify <FILES>",
	Short:         "Test Aidoku packages if they're ready for publishing",
	Version:       rootCmd.Version,
	Args:          cobra.MinimumNArgs(1),
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		zipFiles := common.ProcessGlobs(args)

		filterSchema := gojsonschema.NewStringLoader(verify.FilterSchema())
		sourceSchema := gojsonschema.NewStringLoader(verify.SourceSchema())
		settingsSchema := gojsonschema.NewStringLoader(verify.SettingsSchema())

		errored := false

		for _, file := range zipFiles {
			r, err := zip.OpenReader(file)
			if err != nil {
				color.Red("error: %s is not a valid zip file", file)
				continue
			}
			defer r.Close()

			hasMainWasm := false
			hasLegitIcon := false
			sourceJsonValid := false
			filterJsonValid := true
			settingJsonValid := true
			fmt.Printf("* Testing %s\n", file)
			for _, f := range r.File {
				if f.Name == "Payload/" {
					continue
				}
				fmt.Printf("  * %s\n", strings.TrimPrefix(f.Name, "Payload/"))
				if f.Name == "Payload/main.wasm" {
					hasMainWasm = true
					// TODO: Check if there are enough exported functions
				} else if f.Name == "Payload/Icon.png" {
					rc, err := f.Open()
					if err != nil {
						color.Red("    * error: couldn't read image file for %s: %s", file, err)
						continue
					}
					m, _, err := image.Decode(rc)
					if err != nil {
						color.Red("    * error: could not decode image file for %s: %s", file, err)
						continue
					}
					fmt.Printf("    * Testing if image's dimensions are 128x128... ")
					bounds := m.Bounds()
					w := bounds.Dx()
					h := bounds.Dy()
					if w != 128 && h != 128 {
						color.Red("error: expected 128x128, found %dx%d", w, h)
						continue
					}
					color.Green("ok")

					fmt.Printf("    * Testing if image is fully opaque... ")
					if !opaque(m) {
						color.Red("error")
						continue
					}
					color.Green("ok")

					hasLegitIcon = true
				} else if f.Name == "Payload/source.json" {
					fmt.Printf("    * Testing if source.json is valid against schema... ")
					err = verifySchemas(sourceSchema, f)
					if err == nil {
						sourceJsonValid = true
						color.Green("ok")
						continue
					}
				} else if f.Name == "Payload/settings.json" {
					fmt.Printf("    * Testing if settings.json is valid against schema... ")
					err = verifySchemas(settingsSchema, f)
					if err != nil {
						settingJsonValid = false
						continue
					}
					color.Green("ok")
				} else if f.Name == "Payload/filters.json" {
					fmt.Printf("    * Testing if filters.json is valid against schema... ")
					err = verifySchemas(filterSchema, f)
					if err != nil {
						filterJsonValid = false
						continue
					}
					color.Green("ok")
				}
			}
			if !(hasMainWasm && hasLegitIcon && sourceJsonValid && settingJsonValid && filterJsonValid) {
				if !hasMainWasm {
					color.Red("  * test failed: did not find main.wasm")
				}
				errored = true
			}
			fmt.Printf("\n")
		}

		if errored {
			return errors.New("one or more packages failed validation, see above")
		} else {
			return nil
		}
	},
}

func init() {
	rootCmd.AddCommand(verifyCmd)

	buildCmd.MarkZshCompPositionalArgumentFile(1, "*.aix")
	buildCmd.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"aix"}, cobra.ShellCompDirectiveFilterFileExt
	}
}