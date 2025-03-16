return {
  {
    "kevinhwang91/nvim-ufo",
    opts = {},
    lazy = false,
    init = function()
      -- '0' is not bad
      vim.o.foldcolumn = "1"      
      -- Using ufo provider need a large value, feel free to decrease the value
      vim.o.foldlevel = 99
      vim.o.foldlevelstart = 99
      vim.o.foldenable = true
    end,
    dependencies = { "kevinhwang91/promise-async", "luukvbaal/statuscol.nvim" },
    keys = {
      {
        "zR",
        function()
          require("ufo").openAllFolds()
        end,
        desc = "Open All Folds",
      },
      {
        "zM",
        function()
          require("ufo").closeAllFolds()
        end,
        desc = "Close All Folds",
      },
    },
  },
  -- https://github.com/kevinhwang91/nvim-ufo/issues/4#issuecomment-1512772530
  {
    "luukvbaal/statuscol.nvim",
    config = function()
      local builtin = require("statuscol.builtin")
      require("statuscol").setup({
        relculright = true,
        segments = {
          { text = { builtin.foldfunc }, click = "v:lua.ScFa" },
          { text = { "%s" }, click = "v:lua.ScSa" },
          { text = { builtin.lnumfunc, " " }, click = "v:lua.ScLa" },
        },
      })
    end,
  },
}
