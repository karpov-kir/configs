return {
  "myusuf3/numbers.vim",
  init = function()
    vim.cmd("let g:numbers_exclude = ['lazy', 'neo-tree', 'alpha', 'floaterm', 'toggleterm']")
  end,
}
