# go-gcloud

Golang tools and utilities around google cloud libraries

``` go
import "github.com/blue-army/go-gcloud/emulators"
```

Go package for gcloud services.

To install the packages on your system,

```bash
go get -u github.com/blue-army/go-gcloud
```

## Datastore Emulator

Using the datastore emulator in a test.

```go
package app

import (
    "context"
    "fmt"
    "log"
    "os"
    "testing"
    "time"

    "cloud.google.com/go/datastore"

    gcl "github.com/blue-army/go-gcloud/emulators"
    "github.com/stretchr/testify/assert"
)

var ctx context.Context

func TestMain(m *testing.M) {
    var err error
    var done func()

    done, err = gcl.LaunchDatastoreEmulator(&gcl.DatastoreEmulatorOptions{
        Consistency: "0.1",
    })

    if err != nil {
        fmt.Printf("error: %+v", err)
    }

    ctx = context.Background()
    code := m.Run() // this runs the tests

    done()  // terminate the emulator

    os.Exit(code)
}

func Test_can_add_post(t *testing.T) {

    client, err := datastore.NewClient(ctx, "bad-project-id")  // fail-safe bad project id
    if err != nil {
        log.Fatal(err)
    }

    type Post struct {
        Title       string
        Body        string `datastore:",noindex"`
        PublishedAt time.Time
    }

    keys := []*datastore.Key{
        datastore.NameKey("Post", "post1", nil),
        datastore.NameKey("Post", "post2", nil),
    }
    posts := []*Post{
        {Title: "Post 1", Body: "Galaxy", PublishedAt: time.Now()},
        {Title: "Post 2", Body: "Mosaic", PublishedAt: time.Now()},
    }
    if _, err := client.PutMulti(ctx, keys, posts); err != nil {
        log.Fatal(err)
    }

    fetched := &Post{}

    err = client.Get(ctx, keys[0], fetched)
    if err != nil {
        log.Fatal(err)
    }

    assert.Equal(t, "Galaxy", fetched.Body)
}

```
