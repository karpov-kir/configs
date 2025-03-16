return {
  "vuki656/package-info.nvim",
  opts = {},
  keys = {
    {
      "<leader>ns",
      function()
        -- `force = true` will refetch package infos
        require("package-info").show({ force = true })
      end,
    },
    {
      "<leader>nc",
      function()
        require("package-info").hide()
      end,
    },
    {
      "<leader>nt",
      function()
        require("package-info").toggle()
      end,
    },
    {
      "<leader>nu",
      function()
        require("package-info").update()
      end,
    },
    {
      "<leader>nd",
      function()
        require("package-info").delete()
      end,
    },
    {
      "<leader>ni",
      function()
        require("package-info").install()
      end,
    },
    {
      "<leader>np",
      function()
        require("package-info").change_version()
      end,
    },
  },
  dependencies = {
    "MunifTanjim/nui.nvim"
  }
}
