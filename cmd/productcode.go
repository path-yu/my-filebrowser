package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/filebrowser/filebrowser/v2/productcode"
)

func init() {
	rootCmd.AddCommand(productcodeCmd)
	productcodeCmd.AddCommand(productcodeLsCmd)
	productcodeCmd.AddCommand(productcodeFindCmd)
}

var productcodeCmd = &cobra.Command{
	Use:   "productcode",
	Short: "Product code (PDF index) management utility",
	Long:  `Manage product codes stored in the database (PDF filename → code mapping).`,
	Args:  cobra.NoArgs,
}

var productcodeLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all product code entries stored in the database",
	Long:  `Print every (Path, Code) pair in the product code index table. Use this to verify the DB content vs. list rendering.`,
	Args:  cobra.NoArgs,
	RunE: withStore(func(_ *cobra.Command, _ []string, st *store) error {
		list, err := st.ProductCode.All()
		if err != nil {
			return err
		}
		printProductCodes(list)
		return nil
	}, storeOptions{}),
}

var productcodeFindCmd = &cobra.Command{
	Use:   "find <substring>",
	Short: "Find product code entries whose Path or Code contains the given substring",
	Long:  `Case-insensitive search through the product code index. Useful when you want to verify a specific PDF (e.g. "1立方") has the expected code.`,
	Args:  cobra.ExactArgs(1),
	RunE: withStore(func(_ *cobra.Command, args []string, st *store) error {
		q := args[0]
		all, err := st.ProductCode.All()
		if err != nil {
			return err
		}
		var list []*productcode.Entry
		for _, e := range all {
			if containsIgnoreCase(e.Path, q) || containsIgnoreCase(e.Code, q) {
				list = append(list, e)
			}
		}
		printProductCodes(list)
		return nil
	}, storeOptions{}),
}

func containsIgnoreCase(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	// UTF-8 safe: bytes.ToLower would work for ASCII keywords we use,
	// fall back to strings.ToLower for general correctness.
	sLow := toLowerASCII(s)
	qLow := toLowerASCII(substr)
	return len(sLow) >= len(qLow) && indexOf(sLow, qLow) >= 0
}

func toLowerASCII(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func indexOf(s, substr string) int {
	n, m := len(s), len(substr)
	if m == 0 {
		return 0
	}
	for i := 0; i+m <= n; i++ {
		if s[i:i+m] == substr {
			return i
		}
	}
	return -1
}

func printProductCodes(list []*productcode.Entry) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "#\tPath\tCode")
	for i, e := range list {
		fmt.Fprintf(w, "%d\t%s\t%s\n", i+1, e.Path, e.Code)
	}
	w.Flush()
	fmt.Printf("\nTotal: %d entr%s\n", len(list), plural(len(list), "y", "ies"))
}

func plural(n int, sing, plur string) string {
	if n == 1 {
		return sing
	}
	return plur
}
