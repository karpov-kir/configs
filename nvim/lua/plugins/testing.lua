return {
  "vim-test/vim-test",
  init = function()
    vim.cmd("let test#strategy = 'vimux'")
  end,
  dependencies = {
    "preservim/vimux",
  },
  cmd = {
    "TestNearest",
    "TestFile",
    "TestSuite",
    "TestLast",
    "TestVisit",
  },
  keys = {
    { "<leader>tn", "<cmd>TestNearest<CR>" },
    { "<leader>tf", "<cmd>TestFile<CR>" },
    { "<leader>ts", "<cmd>TestSuite<CR>" },
    { "<leader>tl", "<cmd>TestLast<CR>" },
    { "<leader>tg", "<cmd>TestVisit<CR>" },
  },
}
