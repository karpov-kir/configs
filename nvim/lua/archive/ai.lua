return {
  {
    "github/copilot.vim",
    cmd = "Copilot",
    lazy = false,
    init = function()
      vim.g.copilot_no_tab_map = true
    end,
    keys = {
      { "<c-j>", 'copilot#Accept("")', expr = true, replace_keycodes = false, mode = "i" },
      { "<c-]>", 'copilot#Next()', expr = true, replace_keycodes = false, mode = "i" },
      { "<c-[>", 'copilot#Previous()', expr = true, replace_keycodes = false, mode = "i" },
    },
  },
  {
    "CopilotC-Nvim/CopilotChat.nvim",
    branch = "canary",
    cmd = {
      "CopilotChat",
      "CopilotChatOpen",
      "CopilotChatToggle"
    },
    dependencies = {
      { "github/copilot.vim" },
      { "nvim-lua/plenary.nvim" },
    },
    -- Only on MacOS or Linux
    build = "make tiktoken",
    opts = {},
  },
}
