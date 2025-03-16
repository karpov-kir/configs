return {
  {
    "jay-babu/mason-nvim-dap.nvim",
    dependencies = {
      "williamboman/mason.nvim",
      "mfussenegger/nvim-dap",
    },
    opts = {
      ensure_installed = { "delve" },
    },
  },
  {
    "mfussenegger/nvim-dap",
    dependencies = {
      "leoluz/nvim-dap-go",
    },
    keys = {
      {
        "<leader>b",
        function()
          require("dap").toggle_breakpoint()
        end,
      },
      {
        "<F5>",
        function()
          require("dap").continue()
        end,
      },
      {
        "<F10>",
        function()
          require("dap").step_over()
        end,
      },
      {
        "<F11>",
        function()
          require("dap").step_into()
        end,
      },
      {
        "<F12>",
        function()
          require("dap").step_out()
        end,
      },
    },
    init = function()
      require("dap-go").setup()
    end,
  },
  {
    "rcarriga/nvim-dap-ui",
    init = function()
      local dap = require("dap")
      local dapui = require("dapui")
      dap.listeners.before.attach.dapui_config = function()
        dapui.open()
      end
      dap.listeners.before.launch.dapui_config = function()
        dapui.open()
      end
      dap.listeners.before.event_terminated.dapui_config = function()
        dapui.close()
      end
      dap.listeners.before.event_exited.dapui_config = function()
        dapui.close()
      end
    end,
    opts = {},
    dependencies = {
      "mfussenegger/nvim-dap",
      "nvim-neotest/nvim-nio",
    },
  },
  {
    "theHamsta/nvim-dap-virtual-text",
    dependencies = {
      "mfussenegger/nvim-dap",
    },
  },
}
