return {
  "lukas-reineke/indent-blankline.nvim",
  main = "ibl",
  init = function()
    local space = "·"
    vim.opt.list = true
    vim.opt.listchars:append({
      tab = "│─",
      multispace = space,
      lead = space,
      trail = space,
      nbsp = space,
    })
  end,
  opts = function(_, opts)
    vim.api.nvim_set_hl(0, "ScopeChar", { fg = "#d46ec0" })

    return require("indent-rainbowline").make_opts({
      indent = {
        char = "❘",
      },
      scope = {
        highlight = "ScopeChar",
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
}
