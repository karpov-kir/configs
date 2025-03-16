return {
  "nvim-neo-tree/neo-tree.nvim",
  branch = "v3.x",
  cmd = {
    "Neotree",
  },
  opts = {
    filesystem = {
      filtered_items = {
        visible = true,
      },
    },
  },
  dependencies = {
    "nvim-lua/plenary.nvim",
    "nvim-tree/nvim-web-devicons",
    "MunifTanjim/nui.nvim",
    -- Optional image support in preview window: See `# Preview Mode` for more information
    -- "3rd/image.nvim",
  },
  -- https://github.com/nvim-neo-tree/neo-tree.nvim/issues/1115#issuecomment-1784184617
  keys = function()
    local find_buffer_by_type = function(type)
      for _, buf in ipairs(vim.api.nvim_list_bufs()) do
        local ft = vim.api.nvim_buf_get_option(buf, "filetype")
        if ft == type then
          return buf
        end
      end
      return -1
    end

    local toggle_neotree = function(toggle_command)
      if find_buffer_by_type("neo-tree") > 0 then
        require("neo-tree.command").execute({ action = "close" })
      else
        toggle_command()
      end
    end

    return {
      -- { "<C-n>", "<cmd>Neotree filesystem toggle left<CR>" },
      -- { "<C-n>", "<cmd>Neotree float filesystem toggle<CR>" },
      -- { "<C-n>", "<cmd>Neotree reveal float<CR>" },
      -- { "<C-n>", "<cmd>Neotree reveal left<CR>" },
      {
        "<C-n>",
        function()
          toggle_neotree(function()
            require("neo-tree.command").execute({ action = "focus", reveal = true, dir = vim.uv.cwd() })
          end)
        end,
      },
      -- {
      --   "<leader>E",
      --   function()
      --     toggle_neotree(function()
      --       require("neo-tree.command").execute({ action = "focus", reveal = true })
      --     end)
      --   end,
      --   desc = "Toggle Explorer (root)",
      -- },
    }
  end,
}
