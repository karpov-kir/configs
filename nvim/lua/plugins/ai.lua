return {
  -- A Windsurf version for Vim that also works with Neovim.
  -- `Exafunction/windsurf.nvim` seems to have more features.
  -- {
  --   'Exafunction/windsurf.vim',
  --   event = 'BufEnter'
  -- }
  {
    "Exafunction/windsurf.nvim",
    dependencies = {
      "nvim-lua/plenary.nvim",
    },
    config = function()
      require("codeium").setup({
        enable_cmp_source = false,
        virtual_text = {
          enabled = true,
        },
      })
    end,
  },
}
