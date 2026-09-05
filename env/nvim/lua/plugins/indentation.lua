return {
  -- It's used for rainbow indent + not active indent rendering.
  -- It does not support current indent highlighting (only can highlight the current scope).
  {
    "lukas-reineke/indent-blankline.nvim",
    main = "ibl",
    opts = function(_, opts)
      vim.api.nvim_set_hl(0, "ScopeChar", { fg = "#d46ec0" })

      return require("indent-rainbowline").make_opts({
        indent = {
          char = "❘",
        },
        scope = {
          enabled = false,
        },
      }, {
        color_transparency = 0.045,
      })
    end,
    dependencies = {
      "TheGLander/indent-rainbowline.nvim",
      -- This plugin is required for the scope highlighting to work
      -- You should configure this plugin separately
      "nvim-treesitter/nvim-treesitter",
    },
  },
  -- It's used for active indent highlighting.
  {
    "folke/snacks.nvim",
    opts = {
      indent = {},
    },
    dependencies = {
      "nvim-tree/nvim-web-devicons",
    },
  },
}
