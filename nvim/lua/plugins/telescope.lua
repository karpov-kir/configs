return {
  "nvim-telescope/telescope.nvim",
  branch = "0.1.x",
  dependencies = {
    "nvim-lua/plenary.nvim",
    "nvim-telescope/telescope-ui-select.nvim",
    "nvim-telescope/telescope-dap.nvim",
    "vuki656/package-info.nvim",
    "debugloop/telescope-undo.nvim",
    "folke/noice.nvim",
    {
      "nvim-telescope/telescope-fzf-native.nvim",
      build = "make",
    },
  },
  lazy = false,
  config = function()
    local telescope = require("telescope")
    local actions = require("telescope.actions")
    telescope.setup({
      extensions = {
        ["ui-select"] = {
          require("telescope.themes").get_dropdown({}),
        },
      },
      pickers = {
        buffers = {
          mappings = {
            i = {
              ["<c-r>"] = actions.delete_buffer,
            },
            n = {
              ["<c-r>"] = actions.delete_buffer,
            },
          },
        },
      },
    })
    telescope.load_extension("ui-select")
    telescope.load_extension("dap")
    telescope.load_extension("fzf")
    telescope.load_extension("package_info")
    telescope.load_extension("undo")
    telescope.load_extension("noice")
  end,
  keys = {
    {
      "<C-p>",
      function()
        require("telescope.builtin").find_files()
      end,
    },
    {
      "<leader>fgf",
      function()
        require("telescope.builtin").find_files({ no_ignore = true })
      end,
    },
    {
      "<leader>fgg",
      function()
        require("telescope.builtin").live_grep()
      end,
    },
    {
      "<leader>fb",
      function()
        require("telescope.builtin").buffers()
      end,
    },
    {
      "<leader>fs",
      function()
        require("telescope.builtin").lsp_document_symbols()
      end,
    },
  },
}
