return {
  "ThePrimeagen/harpoon",
  branch = "harpoon2",
  dependencies = { "nvim-lua/plenary.nvim" },
  keys = {
    -- { "<leader>a", function() harpoon:list():add() end },
    { "<leader>sm", function() require("harpoon").ui:toggle_quick_menu(require("harpoon"):list()) end },
  },
}
