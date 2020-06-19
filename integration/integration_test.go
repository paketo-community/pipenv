package integration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudfoundry/dagger"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
)

var (
	bpDir, pythonURI, pipURI, pipenvURI string
)

func TestIntegration(t *testing.T) {
	var Expect = NewWithT(t).Expect

	var err error
	bpDir, err = dagger.FindBPRoot()
	Expect(err).NotTo(HaveOccurred())

	pipenvURI, err = dagger.PackageBuildpack(bpDir)
	Expect(err).ToNot(HaveOccurred())

	pythonURI, err = dagger.GetLatestCommunityBuildpack("paketo-community", "python-runtime")
	Expect(err).ToNot(HaveOccurred())

	pipURI, err = dagger.GetLatestCommunityBuildpack("paketo-community", "pip")
	Expect(err).ToNot(HaveOccurred())

	defer AfterSuite(t)
	spec.Run(t, "Integration", testIntegration, spec.Parallel(), spec.Report(report.Terminal{}))
}

func AfterSuite(t *testing.T) {
	var Expect = NewWithT(t).Expect

	Expect(dagger.DeleteBuildpack(pipenvURI)).To(Succeed())
	Expect(dagger.DeleteBuildpack(pythonURI)).To(Succeed())
	Expect(dagger.DeleteBuildpack(pipURI)).To(Succeed())
}

func testIntegration(t *testing.T, when spec.G, it spec.S) {
	var (
		Expect = NewWithT(t).Expect

		app *dagger.App
	)

	it.After(func() {
		Expect(app.Destroy()).To(Succeed())
	})

	when("building a simple pipenv app without a pipfile lock", func() {
		it("builds and runs", func() {
			var err error
			app, err = dagger.PackBuild(filepath.Join("testdata", "without_pipfile_lock"), pythonURI, pipenvURI, pipURI)
			Expect(err).NotTo(HaveOccurred())

			err = app.Start()
			if err != nil {
				_, err = fmt.Fprintf(os.Stderr, "App failed to start: %v\n", err)
				Expect(err).NotTo(HaveOccurred())
			}

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("Hello, World with pipenv!"))
		})
	})

	when("building a simple pipenv app with a pipfile lock", func() {
		it("builds and runs", func() {
			var err error
			app, err = dagger.PackBuild(filepath.Join("testdata", "pipfile_lock"), pythonURI, pipenvURI, pipURI)
			Expect(err).NotTo(HaveOccurred())

			err = app.Start()
			if err != nil {
				_, err = fmt.Fprintf(os.Stderr, "App failed to start: %v\n", err)
				Expect(err).NotTo(HaveOccurred())
			}

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("Hello, World with pipenv!"))
		})

		it("sets python version to version in pipfile.lock", func() {
			var err error
			app, err = dagger.PackBuild(filepath.Join("testdata", "pipfile_lock"), pythonURI, pipenvURI, pipURI)
			Expect(err).NotTo(HaveOccurred())

			app.SetHealthCheck("", "3s", "1s")
			err = app.Start()
			if err != nil {
				_, err = fmt.Fprintf(os.Stderr, "App failed to start: %v\n", err)
				Expect(err).NotTo(HaveOccurred())
			}

			body, _, err := app.HTTPGet("/")
			Expect(err).ToNot(HaveOccurred())
			Expect(body).To(ContainSubstring("Hello, World with pipenv!"))

			Expect(app.BuildLogs()).To(MatchRegexp(`Installing Python \d+\.\d+\.\d+`))
		})
	})

	when("building a simple pipenv app with a pipfile and requirements.txt", func() {
		it("ignores the pipfile", func() {
			pack := dagger.NewPack(
				filepath.Join("testdata", "pipfile_requirements"),
				dagger.RandomImage(),
				dagger.SetBuildpacks(pythonURI, pipenvURI, pipURI),
				dagger.SetVerbose(),
			)

			_, err := pack.Build()
			Expect(err).To(MatchError(ContainSubstring("found Pipfile + requirements.txt")))
		})
	})

	when("rebuilding a simple pipenv app", func() {
		it("should cache the pipenv binary but not the requirements.txt", func() {
			var err error
			app, err = dagger.PackBuild(filepath.Join("testdata", "pipfile_lock"), pythonURI, pipenvURI, pipURI)
			Expect(err).NotTo(HaveOccurred())

			app.SetHealthCheck("", "3s", "1s")
			err = app.Start()
			Expect(err).ToNot(HaveOccurred())

			_, imgName, _, _ := app.Info()

			app, err = dagger.PackBuildNamedImage(imgName, filepath.Join("testdata", "pipfile_lock"), pythonURI, pipenvURI, pipURI)
			Expect(err).NotTo(HaveOccurred())
			Expect(app.BuildLogs()).ToNot(ContainSubstring("Downloading from https://buildpacks.cloudfoundry.org/dependencies/pipenv/pipenv"))
			Expect(app.BuildLogs()).To(MatchRegexp("Pipenv \\d+\\.\\d+\\.\\d+: Reusing cached layer"))
			Expect(app.BuildLogs()).To(ContainSubstring("Generating requirements.txt from Pipfile.lock"))

			files, err := app.Files(filepath.Join("/workspace", "requirements.txt"))
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(ContainElement(ContainSubstring("requirements.txt")))
		})
	})

	when("when building an app without a pipfile", func() {
		it("should fail during detection", func() {
			pack := dagger.NewPack(
				filepath.Join("testdata", "without_pipfile"),
				dagger.RandomImage(),
				dagger.SetBuildpacks(pythonURI, pipenvURI, pipURI),
				dagger.SetVerbose(),
			)

			_, err := pack.Build()
			Expect(err).To(MatchError(ContainSubstring("no Pipfile found")))
		})
	})
}
