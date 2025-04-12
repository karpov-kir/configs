-- https://alpha2phi.medium.com/modern-neovim-pde-587ef26bb458
return {
  {
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

      -- https://github.com/nvim-telescope/telescope.nvim/issues/2778#issuecomment-2202572413
      -- https://github.com/nvim-telescope/telescope.nvim/issues/2778#issuecomment-2481005686
      local focus_preview = function(prompt_bufnr)
        local action_state = require("telescope.actions.state")
        local picker = action_state.get_current_picker(prompt_bufnr)
        local prompt_win = picker.prompt_win
        local previewer = picker.previewer
        local bufnr = previewer.state.bufnr or previewer.state.termopen_bufnr
        local winid = previewer.state.winid or vim.fn.win_findbuf(bufnr)[1]
        vim.keymap.set("n", "<Tab>", function()
          vim.cmd(string.format("noautocmd lua vim.api.nvim_set_current_win(%s)", prompt_win))
        end, { buffer = bufnr })
        vim.cmd(string.format("noautocmd lua vim.api.nvim_set_current_win(%s)", winid))
        -- api.nvim_set_current_win(winid)
      end

      telescope.setup({
        extensions = {
          ["ui-select"] = {
            require("telescope.themes").get_dropdown({}),
          },
        },
        defaults = {
          mappings = {
            n = {
              ["<Tab>"] = focus_preview,
            },
            i = {
              ["<Tab>"] = focus_preview,
            },
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
  },
  {
    "windwp/nvim-spectre",
    keys = {
      {
        "<leader>sr",
        function()
          require("spectre").open()
        end,
        desc = "Search and Replace (Spectre)",
      },
    },
  },
}
