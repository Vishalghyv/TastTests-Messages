package main

import (
	"context"
	"fmt"
	"time"
	"strings"
	"os"
	"flag"

	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/devtool"
	"github.com/mafredri/cdp/protocol/dom"
	"github.com/mafredri/cdp/protocol/page"
	"github.com/mafredri/cdp/rpcc"
)

type Parameters struct {
	URL, branch, folder string
	commitNum int
	timeout time.Duration
}

func main() {
	var commitNum int
	var url string
	var branch string
	var timeout time.Duration
	var folder string
	flag.IntVar(&commitNum, "commitNum", 10, "the count of items")
	flag.StringVar(&url, "url", "https://chromium.googlesource.com/chromiumos/platform/tast-tests/", "Repository URL")
	flag.StringVar(&branch, "branch", "main", "Branch Name")
	flag.DurationVar(&timeout, "timeout", 5 * time.Second, "Maximum time program to run")
	flag.StringVar(&folder, "folder", "./Commits/", "Folder Path")
	flag.Parse()

	var para = Parameters{commitNum: commitNum, URL: url, branch: branch, timeout: timeout, folder: folder}

	err := run(para)
	if err != nil {
		// log.Fatal(err)
		fmt.Println(err)
	}
}

func run(parameters Parameters) error {
	// Context
	ctx, cancel := context.WithTimeout(context.Background(), parameters.timeout)
	defer cancel()

	// Port used
	// Use the DevTools HTTP/JSON API to manage targets (e.g. pages, webworkers).
	devt := devtool.New("http://127.0.0.1:9222")
	pt, err := devt.Get(ctx, devtool.Page)
	if err != nil {
		pt, err = devt.Create(ctx)
		if err != nil {
			return err
		}
	}

	// Connection
	// Initiate a new RPC connection to the Chrome DevTools Protocol target.
	conn, err := rpcc.DialContext(ctx, pt.WebSocketDebuggerURL)
	if err != nil {
		return err
	}
	defer conn.Close() // Leaving connections open will leak memory.

	c := cdp.NewClient(conn)

	// Open a DOMContentEventFired client to buffer this event.
	domContent, err := c.Page.DOMContentEventFired(ctx)
	if err != nil {
		return err
	}
	defer domContent.Close()

	// Enable events on the Page domain, it's often preferrable to create
	// event clients before enabling events so that we don't miss any.
	if err = c.Page.Enable(ctx); err != nil {
		return err
	}

	// Navigating to main site
	// Create the Navigate arguments with the optional Referrer field set.

	OpenDoc := func (URL string, Referrer string) (*dom.GetDocumentReply, error) {
		navArgs := page.NewNavigateArgs(URL).
			SetReferrer(Referrer)
	
		nav, err := c.Page.Navigate(ctx, navArgs)
		if err != nil {
			return nil, err
		}
	
		// Wait until we have a DOMContentEventFired event.
		if _, err = domContent.Recv(); err != nil {
			return nil, err
		}
	
		fmt.Printf("Page loaded with frame ID: %s\n", nav.FrameID)
	
	
		doc, err := c.DOM.GetDocument(ctx, nil)
		
		return doc, err
	}

	
	Link := parameters.URL
	doc, err := OpenDoc(Link, "https://google.com")
	if err != nil {
		return err
	}

	// Get link for main branch

	// Get the outer HTML for the page.
	result, err := c.DOM.GetOuterHTML(ctx, &dom.GetOuterHTMLArgs{
		NodeID: &doc.Root.NodeID,
	})
	if err != nil {
		return err
	}

	searchId,err := c.DOM.PerformSearch(ctx, &dom.PerformSearchArgs{
		Query: parameters.branch,
	})
	if err != nil {
		return err
	}

	nodeIds,err := c.DOM.GetSearchResults(ctx, &dom.GetSearchResultsArgs{
		SearchID: searchId.SearchID,
		FromIndex: 0,
		ToIndex: searchId.ResultCount,
	})
	if err != nil {
		return err
	}

	var branch string

	for _, value := range nodeIds.NodeIDs {
		// fmt.Printf("- %d\n", value)
		result, err := c.DOM.GetOuterHTML(ctx, &dom.GetOuterHTMLArgs{
			NodeID: &value,
		})
		if err != nil {
			fmt.Printf("Err : %s\n", err)
			continue;
		}
	
		fmt.Printf("First Link: %s\n", result.OuterHTML)
		att, err := c.DOM.GetAttributes(ctx, &dom.GetAttributesArgs{
			NodeID: value,
		})

		if err != nil|| len(att.Attributes) == 0{
			continue;
		}

		branch = att.Attributes[len(att.Attributes)-1]
		break;
	  }
	fmt.Println("Branch", branch)
	  
	Link ="https://chromium.googlesource.com" + branch

	doc, err = OpenDoc(Link, "https://google.com")
	if err != nil {
		return err
	}

	QuerySelector := func (NodeID dom.NodeID, Selector string) (dom.NodeID, error) {
		message, err := c.DOM.QuerySelector(ctx, &dom.QuerySelectorArgs{
			NodeID: NodeID,
			Selector: Selector,
		})
	
		return message.NodeID, err
	}

	QueryHTML := func (NodeID dom.NodeID, Selector string) (string, error) {
		messageID, err := QuerySelector(NodeID, Selector)

		if err != nil {
			return "", err
		}

		result, err = c.DOM.GetOuterHTML(ctx, &dom.GetOuterHTMLArgs{
			NodeID: &messageID,
		})

		return result.OuterHTML, err

	}

	html, err := QueryHTML(doc.Root.NodeID, ".u-monospace.Metadata td")

		if err != nil {
			return err
		}

	next := strings.TrimLeft(strings.TrimRight(html,"</td>"), "<td>")
	fmt.Println("Next fuck", next)


	//OKAY

	//From Next Commit

	for i := 1; i <= parameters.commitNum; i++ {
		Link :="https://chromium.googlesource.com/chromiumos/platform/tast-tests/+/" + next

		doc, err = OpenDoc(Link, "https://google.com")
		if err != nil {
			return err
		}

		html, err = QueryHTML(doc.Root.NodeID, ".MetadataMessage")

		if err != nil {
			return err
		}

		name := strings.Split(strings.Split(html, "\n")[0], ">")[1]

		fmt.Printf("Commit Message: %s\n", name)

		html, err = QueryHTML(doc.Root.NodeID, "a[href*='/chromiumos/platform/tast-tests/+/" + next + "%5E']")

		if err != nil {
			return err
		}

		out := strings.TrimLeft(strings.TrimRight(html,"</a>"),"<a>")
		
		textFile := parameters.folder + "./Commits" + next[0:6] + ".txt"
		err = CreateFile(textFile, name)

		if err != nil {
			return err
		}

		next = strings.Split(out, ">")[1]
		fmt.Println("Next ", next)

	}
	return nil
}


func CreateFile(fileName string, message string) error {
	f, err := os.Create(fileName)

    if err != nil {
        return err
    }

    defer f.Close()

    _, err2 := f.WriteString(message)

    if err2 != nil {
        return err2
    }
	return nil

}