return {
  "voldikss/vim-floaterm",
  keys = {
    { "<Leader>ft", "<cmd>FloatermToggle<CR>" },
    { "<Leader>fc", "<cmd>FloatermNew<CR>" },
    { "<Leader>fp", "<cmd>FloatermPrev<CR>" },
    { "<Leader>fn", "<cmd>FloatermNext<CR>" },
  },
  init = function()
    -- Do not use this, because then the leader is mapped to space in the terminal mode
    -- and there is a delay when pressing space, because it waits for the next key
    -- vim.cmd("let g:floaterm_keymap_toggle = '<Leader>ft'")
    -- vim.cmd("let g:floaterm_keymap_new = '<Leader>fc'")
    -- vim.cmd("let g:floaterm_keymap_prev = '<Leader>fp'")
    -- vim.cmd("let g:floaterm_keymap_next = '<Leader>fn'")

    -- Apply the highlight when the terminal is opened
    -- autocmd TermOpen * highlight TermCursor ctermfg=white ctermbg=yellow guifg=blue guibg=yellow
    vim.cmd([[
      augroup FloatermSettings
        autocmd!
        autocmd TermOpen * setlocal cursorline
        autocmd TermOpen * highlight TermCursor ctermfg=white guifg=blue
      augroup END
    ]])
  end,
}
