package etchosts

import (
	"bytes"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sync/errgroup"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestBuildDefault(t *testing.T) {
	file, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	// check that /etc/hosts has consistent ordering
	for i := 0; i <= 5; i++ {
		err = Build(file.Name(), nil)
		if err != nil {
			t.Fatal(err)
		}

		content, err := os.ReadFile(file.Name())
		if err != nil {
			t.Fatal(err)
		}
		expected := "127.0.0.1\tlocalhost\n::1\tlocalhost ip6-localhost ip6-loopback\nfe00::\tip6-localnet\nff00::\tip6-mcastprefix\nff02::1\tip6-allnodes\nff02::2\tip6-allrouters\n"

		actual := string(content)
		if expected != actual {
			assert.Check(t, is.Equal(actual, expected))
		}
	}
}

func TestBuildNoIPv6(t *testing.T) {
	d := t.TempDir()
	filename := filepath.Join(d, "hosts")

	err := BuildNoIPv6(filename, []Record{
		{
			Hosts: "another.example",
			IP:    netip.MustParseAddr("fdbb:c59c:d015::3"),
		},
		{
			Hosts: "another.example",
			IP:    netip.MustParseAddr("10.11.12.13"),
		},
	})
	assert.NilError(t, err)
	content, err := os.ReadFile(filename)
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(string(content), "127.0.0.1\tlocalhost\n10.11.12.13\tanother.example\n"))
}

func TestUpdate(t *testing.T) {
	file, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	if err := Build(file.Name(), []Record{
		{
			"testhostname.testdomainname testhostname",
			netip.MustParseAddr("10.11.12.13"),
		},
	}); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "10.11.12.13\ttesthostname.testdomainname testhostname\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}

	if err := Update(file.Name(), "1.1.1.1", "testhostname"); err != nil {
		t.Fatal(err)
	}

	content, err = os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "1.1.1.1\ttesthostname.testdomainname testhostname\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}

// This regression test ensures that when a host is given a new IP
// via the Update function that other hosts which start with the
// same name as the targeted host are not erroneously updated as well.
// In the test example, if updating a host called "prefix", unrelated
// hosts named "prefixAndMore" or "prefix2" or anything else starting
// with "prefix" should not be changed. For more information see
// GitHub issue #603.
func TestUpdateIgnoresPrefixedHostname(t *testing.T) {
	file, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	if err := Build(file.Name(), []Record{
		{
			Hosts: "prefix",
			IP:    netip.MustParseAddr("2.2.2.2"),
		},
		{
			Hosts: "prefixAndMore",
			IP:    netip.MustParseAddr("3.3.3.3"),
		},
		{
			Hosts: "unaffectedHost",
			IP:    netip.MustParseAddr("4.4.4.4"),
		},
	}); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "2.2.2.2\tprefix\n3.3.3.3\tprefixAndMore\n4.4.4.4\tunaffectedHost\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}

	if err := Update(file.Name(), "5.5.5.5", "prefix"); err != nil {
		t.Fatal(err)
	}

	content, err = os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "5.5.5.5\tprefix\n3.3.3.3\tprefixAndMore\n4.4.4.4\tunaffectedHost\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}

// This regression test covers the host prefix issue for the
// Delete function. In the test example, if deleting a host called
// "prefix", an unrelated host called "prefixAndMore" should not
// be deleted. For more information see GitHub issue #603.
func TestDeleteIgnoresPrefixedHostname(t *testing.T) {
	file, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := Add(file.Name(), []Record{
		{
			Hosts: "prefix",
			IP:    netip.MustParseAddr("1.1.1.1"),
		},
		{
			Hosts: "prefixAndMore",
			IP:    netip.MustParseAddr("2.2.2.2"),
		},
	}); err != nil {
		t.Fatal(err)
	}

	if err := Delete(file.Name(), []Record{
		{
			Hosts: "prefix",
			IP:    netip.MustParseAddr("1.1.1.1"),
		},
	}); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "2.2.2.2\tprefixAndMore\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}

	if expected := "1.1.1.1\tprefix\n"; bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Did not expect to find '%s' got '%s'", expected, content)
	}
}

func TestAddEmpty(t *testing.T) {
	file, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := Add(file.Name(), []Record{}); err != nil {
		t.Fatal(err)
	}
}

func TestAdd(t *testing.T) {
	file, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := Add(file.Name(), []Record{
		{
			Hosts: "testhostname",
			IP:    netip.MustParseAddr("2.2.2.2"),
		},
	}); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "2.2.2.2\ttesthostname\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}

func TestDeleteEmpty(t *testing.T) {
	file, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := Delete(file.Name(), []Record{}); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteNewline(t *testing.T) {
	file, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	b := []byte("\n")
	if _, err := file.Write(b); err != nil {
		t.Fatal(err)
	}

	rec := []Record{
		{
			Hosts: "prefix",
			IP:    netip.MustParseAddr("2.2.2.2"),
		},
	}
	if err := Delete(file.Name(), rec); err != nil {
		t.Fatal(err)
	}
}

func TestDelete(t *testing.T) {
	file, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := Add(file.Name(), []Record{
		{
			Hosts: "testhostname1",
			IP:    netip.MustParseAddr("1.1.1.1"),
		},
		{
			Hosts: "testhostname2",
			IP:    netip.MustParseAddr("2.2.2.2"),
		},
		{
			Hosts: "testhostname3",
			IP:    netip.MustParseAddr("3.3.3.3"),
		},
	}); err != nil {
		t.Fatal(err)
	}

	if err := Delete(file.Name(), []Record{
		{
			Hosts: "testhostname1",
			IP:    netip.MustParseAddr("1.1.1.1"),
		},
		{
			Hosts: "testhostname3",
			IP:    netip.MustParseAddr("3.3.3.3"),
		},
	}); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "2.2.2.2\ttesthostname2\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}

	if expected := "1.1.1.1\ttesthostname1\n"; bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Did not expect to find '%s' got '%s'", expected, content)
	}
}

func TestConcurrentWrites(t *testing.T) {
	file, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	err = Build(file.Name(), nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := Add(file.Name(), []Record{
		{
			Hosts: "inithostname",
			IP:    netip.MustParseAddr("172.17.0.1"),
		},
	}); err != nil {
		t.Fatal(err)
	}

	group := new(errgroup.Group)
	for i := byte(0); i < 10; i++ {
		group.Go(func() error {
			addr, ok := netip.AddrFromSlice([]byte{i, i, i, i})
			assert.Assert(t, ok)
			rec := []Record{
				{
					IP:    addr,
					Hosts: fmt.Sprintf("testhostname%d", i),
				},
			}

			for j := 0; j < 25; j++ {
				if err := Add(file.Name(), rec); err != nil {
					return err
				}

				if err := Delete(file.Name(), rec); err != nil {
					return err
				}
			}
			return nil
		})
	}

	if err := group.Wait(); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}

	if expected := "172.17.0.1\tinithostname\n"; !bytes.Contains(content, []byte(expected)) {
		t.Fatalf("Expected to find '%s' got '%s'", expected, content)
	}
}

func benchDelete(b *testing.B) {
	b.StopTimer()
	file, err := os.CreateTemp("", "")
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		b.StopTimer()
		file.Close()
		os.Remove(file.Name())
		b.StartTimer()
	}()

	err = Build(file.Name(), nil)
	if err != nil {
		b.Fatal(err)
	}

	var records []Record
	var toDelete []Record
	for i := byte(0); i < 255; i++ {
		addr, ok := netip.AddrFromSlice([]byte{i, i, i, i})
		assert.Assert(b, ok)
		record := Record{
			Hosts: fmt.Sprintf("testhostname%d", i),
			IP:    addr,
		}
		records = append(records, record)
		if i%2 == 0 {
			toDelete = append(records, record)
		}
	}

	if err := Add(file.Name(), records); err != nil {
		b.Fatal(err)
	}

	b.StartTimer()
	if err := Delete(file.Name(), toDelete); err != nil {
		b.Fatal(err)
	}
}

func BenchmarkDelete(b *testing.B) {
	for i := 0; i < b.N; i++ {
		benchDelete(b)
	}
}
