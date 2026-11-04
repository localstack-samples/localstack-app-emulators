package gaps

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
)

// Write prints the report as a table, gaps first, then the app emulator
// releases it was made against.
func Write(w io.Writer, r Report) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "COVERAGE\tCALL\tCOUNT\tFAILED\tBASIS")
	for _, c := range r.Calls {
		fmt.Fprintf(tw, "%s\t%s:%s %s\t%d\t%d\t%s\n", c.Coverage, c.Vendor, c.Method, c.Path, c.Count, c.Failed, c.Basis)
	}
	tw.Flush()

	fmt.Fprintf(w, "\n%d distinct calls, %d not covered\n", len(r.Calls), r.Gaps())
	vendors := make([]string, 0, len(r.Versions))
	for v := range r.Versions {
		vendors = append(vendors, v)
	}
	sort.Strings(vendors)
	for _, v := range vendors {
		fmt.Fprintf(w, "%s app emulator: %s\n", v, strings.Join(r.Versions[v], ", "))
	}
	fmt.Fprintln(w, "A resource-level basis cannot tell which operations of a resource are implemented.")
}
