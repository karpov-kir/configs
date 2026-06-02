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
      { "<c-]>", "copilot#Next()", expr = true, replace_keycodes = false, mode = "i" },
      { "<c-[>", "copilot#Previous()", expr = true, replace_keycodes = false, mode = "i" },
    },
  },
  {
    "CopilotC-Nvim/CopilotChat.nvim",
    branch = "canary",
    cmd = {
      "CopilotChat",
      "CopilotChatOpen",
      "CopilotChatToggle",
    },
    dependencies = {
      { "github/copilot.vim" },
      { "nvim-lua/plenary.nvim" },
    },
    -- Only on MacOS or Linux
    build = "make tiktoken",
    opts = {},
  },
  {
    "yetone/avante.nvim",
    event = "VeryLazy",
    version = false,
    opts = {
      provider = "openai",
      openai = {
        endpoint = "https://api.openai.com/v1",
        model = "gpt-4o",
        timeout = 30000,
        temperature = 0,
        max_tokens = 8192,
      },
    },
    -- if you want to build from source then do `make BUILD_FROM_SOURCE=true`
    build = "make",
    dependencies = {
      "nvim-treesitter/nvim-treesitter",
      "stevearc/dressing.nvim",
      "nvim-lua/plenary.nvim",
      "MunifTanjim/nui.nvim",
      "nvim-telescope/telescope.nvim",
      "nvim-tree/nvim-web-devicons",
    },
  },
}
