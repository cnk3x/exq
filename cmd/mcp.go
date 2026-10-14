package cmd

import (
	"context"

	"exq/mcp"

	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "作为 MCP server 运行（stdio）",
	Long: `以 Model Context Protocol server 模式运行，通过 stdio 提供以下工具：

  excel_get     按坐标取值
  excel_select  按 CEL 表达式筛选行

客户端配置示例：
  {
    "mcpServers": {
      "exq": { "command": "exq", "args": ["mcp"] }
    }
  }`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return mcp.Run(context.Background())
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}
