-- NOTE: `vtsls` does not support automatic renaming of imports when file names are changed or file paths are changed (e.g. via Yazi in Neovim).
-- It was found in the LSP's logs: `{ "error": { "code": -32601, "message": "Unhandled method workspace/willRenameFiles" } }`.
local lsp_servers = { "lua_ls", "ts_ls", "gopls" }

return {
  -- LSP
  {
    "williamboman/mason.nvim",
    opts = {},
  },
  {
    "williamboman/mason-lspconfig.nvim",
    dependencies = { "williamboman/mason.nvim" },
    opts = { ensure_installed = lsp_servers },
  },
  {
    "WhoIsSethDaniel/mason-tool-installer.nvim",
    dependencies = { "williamboman/mason.nvim" },
    opts = {
      ensure_installed = {
        "eslint_d",
        "stylua",
        "selene",
        "prettier",
      },
    },
  },
  {
    "neovim/nvim-lspconfig",
    dependencies = { "williamboman/mason-lspconfig.nvim", "saghen/blink.cmp" },
    init = function()
      -- vim.lsp.set_log_level("debug")
    end,
    config = function()
      local lspconfig = require("lspconfig")
      local capabilities = require("blink.cmp").get_lsp_capabilities()

      for _, lsp_server in ipairs(lsp_servers) do
        local config = {
          capabilities = capabilities,
        }
  
        -- Can be removed if `vtsls` is not used, but let's keep it for now
        if lsp_server == "vtsls" then
          config.settings = {
            vtsls = { 
              autoUseWorkspaceTsdk = true
            }
          }
        end

        lspconfig[lsp_server].setup(config)
      end
    end,
    keys = {
      -- [From the plugin's README.md]
      -- Nvim sets some default options and mappings when a buffer attaches to LSP (see :help lsp-config). In particular:
      -- 'tagfunc'
      --   Enables "go to definition" capabilities using <C-]> and other tag commands.
      -- 'omnifunc'
      --   Enables (manual) omni mode completion with <C-X><C-O> in Insert mode. For autocompletion, an autocompletion plugin is required.
      -- 'formatexpr'
      --   Enables LSP formatting with gq.
      -- K maps to vim.lsp.buf.hover() in Normal mode.
      -- [d and ]d map to vim.diagnostic.goto_prev() and vim.diagnostic.goto_next(), respectively.
      -- <C-W>d maps to vim.diagnostic.open_float().

      -- { "K", vim.lsp.buf.hover },
      { "<leader>gd", vim.lsp.buf.definition },
      { "<leader>gr", vim.lsp.buf.references },
      { "<leader>rn", vim.lsp.buf.rename },
      { "<leader>ca", vim.lsp.buf.code_action, mode = { "n", "v" } },
    },
  },

  -- LINTERS
  {
    "mfussenegger/nvim-lint",
    init = function()
      require("lint").linters_by_ft = {
        lua = { "selene" },
        javascript = { "eslint_d" },
        typescript = { "eslint_d" },
      }

      vim.api.nvim_create_autocmd({ "BufWritePost", "BufReadPost", "InsertLeave" }, {
        callback = function()
          -- `try_lint` without arguments runs the linters defined in `linters_by_ft`
          -- for the current filetype
          require("lint").try_lint()

          -- You can call `try_lint` with a linter name or a list of names to always
          -- run specific linters, independent of the `linters_by_ft` configuration
          -- require("lint").try_lint("cspell")
        end,
      })
    end,
  },

  -- FORMATTERS
  {
    "stevearc/conform.nvim",
    opts = {
      formatters_by_ft = {
        lua = { "stylua" },
        javascript = { "eslint_d", "prettier", stop_after_first = true },
        typescript = { "eslint_d", "prettier", stop_after_first = true },
      },
    },
    keys = {
      {
        "<leader>ff",
        function()
          require("conform").format()
        end,
        desc = "Format file"
      },
    },
  },
}
