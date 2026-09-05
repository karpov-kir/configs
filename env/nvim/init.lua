-- Copy from Neovim to the system clipboard
-- https://www.reddit.com/r/neovim/comments/3fricd/comment/i1mnu1a
vim.api.nvim_set_option("clipboard", "unnamed")
-- TODO find out what is mapped to <ESC> in insert mode (without this <ESC> does not exit insert mode)
vim.cmd("inoremap <ESC> <ESC>")

-- Window navigation and resizing. These used to ride along with zellij-nav.nvim, which went out
-- with Zellij itself; <C-w> does the navigation half natively.
vim.keymap.set("n", "<C-h>", "<C-w>h", { silent = true, desc = "navigate left" })
vim.keymap.set("n", "<C-j>", "<C-w>j", { silent = true, desc = "navigate down" })
vim.keymap.set("n", "<C-k>", "<C-w>k", { silent = true, desc = "navigate up" })
vim.keymap.set("n", "<C-l>", "<C-w>l", { silent = true, desc = "navigate right" })
vim.keymap.set("n", "<M-h>", "<cmd>vertical resize -5<cr>", { silent = true, desc = "resize left" })
vim.keymap.set("n", "<M-j>", "<cmd>resize -5<cr>", { silent = true, desc = "resize down" })
vim.keymap.set("n", "<M-k>", "<cmd>resize +5<cr>", { silent = true, desc = "resize up" })
vim.keymap.set("n", "<M-l>", "<cmd>vertical resize +5<cr>", { silent = true, desc = "resize right" })

require("config.lazy")
