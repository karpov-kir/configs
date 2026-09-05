return {
  {
    "lewis6991/gitsigns.nvim",
    opts = {},
    lazy = false,
    keys = {
      {
        "<leader>hp",
        function()
          require("gitsigns").preview_hunk()
        end,
      },
      {
        "<leader>tb",
        function()
          require("gitsigns").toggle_current_line_blame()
        end,
      },
    },
  },
}
