package emulators

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"time"
)

var (
	datastoreEmulatorHostRE = regexp.MustCompile(`export DATASTORE_EMULATOR_HOST=(\S+)`)
)

// emulator represents a running instance of an emulator.
type emulator interface {
	// Close kills the emulator.
	io.Closer
}

// DatastoreEmulatorOptions is used to specify options when creating an Instance.
// --host=localhost --port=8624 --store_on_disk=True --consistency=0.9
type DatastoreEmulatorOptions struct {
	Consistency string
}

// LaunchDatastoreEmulator starts a new instance of the datstore emulator,
// sets the appropriate environment variables, and returns a
// closure that must be called when the emulator is no longer required.
func LaunchDatastoreEmulator(opts *DatastoreEmulatorOptions) (func(), error) {

	var emu emulator

	done := func() {
		if emu != nil {
			emu.Close()
		}
	}

	// create instance
	emu, err := newInstance(opts)

	return done, err
}

// instance implements the Instance interface.
type instance struct {
	opts     *DatastoreEmulatorOptions
	child    *exec.Cmd
	apiURL   *url.URL // base URL of API HTTP server
	adminURL string   // base URL of admin HTTP server
	relFuncs []func() // funcs to release any associated contexts
}

func newInstance(opts *DatastoreEmulatorOptions) (emulator, error) {

	// initialize
	i := &instance{
		opts: opts,
	}

	if err := i.startChild(); err != nil {
		return nil, err
	}

	return i, nil
}

func (i *instance) Close() (err error) {

	// check child process
	if i.child == nil {
		return nil
	}

	// defer reset child
	defer func() {
		i.child = nil
	}()

	// kill process
	if p := i.child.Process; p != nil {

		err := p.Kill()
		if err != nil {
			return err
		}
	}
	return
}

func (i *instance) startChild() (err error) {

	datastoreEmulatorArgs := []string{
		"beta",
		"emulators",
		"datastore",
		"start",
		"--consistency=" + i.opts.Consistency,
	}

	i.child = exec.Command("gcloud",
		datastoreEmulatorArgs...,
	)
	i.child.Stdout = os.Stdout
	var stderr io.Reader
	stderr, err = i.child.StderrPipe()
	if err != nil {
		return err
	}
	stderr = io.TeeReader(stderr, os.Stderr)
	if err = i.child.Start(); err != nil {
		return err
	}

	// Read stderr until we have read the URLs of the API server and admin interface.
	errc := make(chan error, 1)
	go func() {
		s := bufio.NewScanner(stderr)
		for s.Scan() {
			if match := datastoreEmulatorHostRE.FindStringSubmatch(s.Text()); match != nil {
				u, err := url.Parse(match[1])
				if err != nil {
					errc <- fmt.Errorf("failed to parse API URL %q: %v", match[1], err)
					return
				}
				i.apiURL = u
			}
			if i.apiURL != nil {
				break
			}
		}
		errc <- s.Err()
	}()

	select {
	case <-time.After(15 * time.Second):
		if p := i.child.Process; p != nil {
			p.Kill()
		}
		return errors.New("timeout starting child process")
	case err := <-errc:
		if err != nil {
			return fmt.Errorf("error reading child process stderr: %v", err)
		}
	}

	if i.apiURL == nil {
		return errors.New("unable to find API server URL")
	}

	fmt.Printf("datastore emulator host: %s", i.apiURL.String())
	os.Setenv("DATASTORE_EMULATOR_HOST", i.apiURL.String())

	return nil
}
