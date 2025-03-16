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
  {
    "kdheepak/lazygit.nvim",
    cmd = {
      "LazyGit",
      "LazyGitConfig",
      "LazyGitCurrentFile",
      "LazyGitFilter",
      "LazyGitFilterCurrentFile",
    },
    -- optional for floating window border decoration
    dependencies = {
      "nvim-lua/plenary.nvim",
    },
    init = function()
      vim.cmd("let g:lazygit_floating_window_use_plenary = 1")
    end,
    -- setting the keybinding for LazyGit with 'keys' is recommended in
    -- order to load the plugin when the command is run for the first time
    keys = {
      { "<leader>lg", "<cmd>LazyGit<cr>", desc = "LazyGit" },
    },
  },
}
