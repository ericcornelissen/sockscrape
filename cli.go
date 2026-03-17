// Copyright (C) 2026  Eric Cornelissen
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
)

var browserOptions = playwright.BrowserTypeLaunchOptions{
	Headless: playwright.Bool(false),
	Devtools: playwright.Bool(false),
}

var (
	ecosystems = []string{
		"actions",
		"cargo",
		"chrome",
		"composer",
		"go",
		"huggingface",
		"maven",
		"npm",
		"nuget",
		"openvsx",
		"pypi",
		"rubygems",
	}
)

var (
	flagInstall = flag.Bool(
		"install",
		false,
		"whether to install playwright",
	)
	flagEcosystem = flag.String(
		"ecosystem",
		"",
		"one of '"+strings.Join(ecosystems, "','")+"'",
	)
	flagModule = flag.String(
		"module",
		"",
		"a module/package/crate name (e.g., 'sha-rust')",
	)
	flagVersion = flag.String(
		"version",
		"",
		"a specific version to get",
	)
)

type (
	version string
	Target  struct {
		ecosystem, module string
	}
)

func main() {
	if err := flags(); err != nil {
		fmt.Printf("Usage error: %v\n", err)
		os.Exit(2)
	}

	if *flagInstall {
		if err := playwright.Install(); err != nil {
			fmt.Println("Could not install PlayWright")
			os.Exit(1)
		}

		os.Exit(0)
	}

	target := Target{
		ecosystem: *flagEcosystem,
		module:    *flagModule,
	}

	browser, err := start()
	if err != nil {
		fmt.Printf("could not set up: %v\n", err)
		os.Exit(1)
	}

	var versions []version
	if *flagVersion != "" {
		versions = []version{version(*flagVersion)}
	} else {
		versions, err = getVersions(browser, target)
		if err != nil {
			fmt.Printf("could not get versions: %v\n", err)
			os.Exit(1)
		}
	}

	var wg sync.WaitGroup
	for _, version := range versions {
		wg.Go(func() {
			if err := downloadVersion(browser, &target, version); err != nil {
				fmt.Printf("[WARN] Could not download %q: %v\n", version, err)
			}
		})
	}
	wg.Wait()

	fmt.Printf("[INFO] Finished downloading %s\n", target.module)
}

func flags() error {
	flag.Parse()

	if *flagInstall {
		return nil
	}

	if *flagEcosystem == "" {
		return fmt.Errorf("must provide an -ecosystem <%s>", strings.Join(ecosystems, "|"))
	}

	if !slices.Contains(ecosystems, *flagEcosystem) {
		return fmt.Errorf("unknown ecosystem %q (want <%s>)", *flagEcosystem, strings.Join(ecosystems, "|"))
	}

	if *flagModule == "" {
		return errors.New("must provide a -module <NAME>")
	}

	return nil
}

func start() (playwright.BrowserContext, error) {
	fmt.Println("[INFO] Starting browser...")

	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}

	browser, err := pw.Chromium.Launch(browserOptions)
	if err != nil {
		return nil, err
	}

	context, err := browser.NewContext()
	if err != nil {
		return nil, err
	}

	return context, nil
}

func getVersions(browser playwright.BrowserContext, target Target) ([]version, error) {
	fmt.Println("[INFO] Obtaining version list...")

	page, err := browser.NewPage()
	if err != nil {
		return nil, err
	}
	defer page.Close()

	url := fmt.Sprintf("https://socket.dev/%s/package/%s/overview", target.ecosystem, target.module)
	if _, err := page.Goto(url); err != nil {
		return nil, err
	}

	locator := page.Locator("option")
	if err := locator.Err(); err != nil {
		return nil, fmt.Errorf("could not find list of versions: %v", err)
	}

	texts, err := locator.AllInnerTexts()
	if err != nil {
		return nil, fmt.Errorf("could not get inner texts: %v", err)
	}

	if len(texts) == 0 {
		return nil, fmt.Errorf("no versions found")
	}

	versions := make([]version, len(texts))
	for i, v := range texts {
		v = strings.Replace(v, " (latest)", "", 1)
		v = strings.Replace(v, " unpublished", "", 1)
		versions[i] = version(v)
	}

	return versions, nil
}

func downloadVersion(browser playwright.BrowserContext, target *Target, v version) error {
	fmt.Printf("[INFO] Downloading %s...\n", v)

	page, err := browser.NewPage()
	if err != nil {
		return err
	}
	defer page.Close()

	url := fmt.Sprintf("https://socket.dev/%s/package/%s/files/%s", target.ecosystem, target.module, v)
	if _, err := page.Goto(url); err != nil {
		return err
	}

	locator := page.Locator("[data-testid='file-explorer']")
	err = locator.WaitFor( playwright.LocatorWaitForOptions{
			State:   playwright.WaitForSelectorStateVisible,
			Timeout: playwright.Float(5_000),
	})
	if err != nil {
			return err
	}

	path := filepath.Join(".", "out", string(v))
	if err := os.Mkdir(path, 0755); err != nil {
		return fmt.Errorf("failed to create dir %q: %v", path, err)
	}

	if err := downloadDirRecursive(page, v, path); err != nil {
		return err
	}

	fmt.Printf("[INFO] Finished downloading %s\n", v)
	return nil
}

func downloadDirRecursive(page playwright.Page, v version, path string) error {
	items, err := page.GetByTestId("file-explorer/file-item").All()
	if err != nil {
		return fmt.Errorf("could not get items: %v\n", err)
	} else if len(items) == 0 {
		return errors.New("no items found")
	}

	for _, item := range items {
		name, err := item.Locator("p").First().TextContent()
		if err != nil {
			return fmt.Errorf("could not get item name: %v", err)
		}

		ty, err := item.GetAttribute("data-type")
		if err != nil {
			return fmt.Errorf("could not get item type: %v", err)
		}

		if ty == "up" {
			continue
		}

		fmt.Printf("[INFO] %s, Downloading %s %q\n", v, ty, name)

		switch ty {
		case "dir":
			if err := item.Click(); err != nil {
				return fmt.Errorf("could not click into dir %q: %v", name, err)
			}

			time.Sleep(500 * time.Millisecond)

			path := filepath.Join(path, name)
			if err := os.Mkdir(path, 0755); err != nil {
				return fmt.Errorf("failed to create dir %q: %v", name, err)
			}

			if err := downloadDirRecursive(page, v, path); err != nil {
				return fmt.Errorf("failed to download %q: %v", name, err)
			}

			if err := page.GetByTitle("..").First().Click(); err != nil {
				return fmt.Errorf("could not click out of dir %q: %v", name, err)
			}

			time.Sleep(500 * time.Millisecond)
		case "file":
			if err := item.Click(); err != nil {
				return fmt.Errorf("could not click into dir %q: %v", name, err)
			}

			time.Sleep(500 * time.Millisecond)

			download, err := page.ExpectDownload(func() error {
				return page.GetByTestId("file-explorer/download-button").Click()
			})
			if err != nil {
				text, _ := item.TextContent()
				return fmt.Errorf("could not download %q: %v", text, err)
			}

			if err := download.SaveAs(filepath.Join(path, name)); err != nil {
				return fmt.Errorf("could not save %q: %v", name, err)
			}

			if _, err := page.GoBack(); err != nil {
				return fmt.Errorf("could not click out of dir %q: %v", name, err)
			}

			time.Sleep(500 * time.Millisecond)
		default:
			return fmt.Errorf("unknown item type %q", ty)
		}
	}

	return nil
}
