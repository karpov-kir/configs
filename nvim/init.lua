-- Copy from Neovim to the system clipboard
-- https://www.reddit.com/r/neovim/comments/3fricd/comment/i1mnu1a
vim.api.nvim_set_option("clipboard", "unnamed")
-- TODO find out what is mapped to <ESC> in insert mode (without this <ESC> does not exit insert mode)
vim.cmd("inoremap <ESC> <ESC>")
require("config.lazy")
