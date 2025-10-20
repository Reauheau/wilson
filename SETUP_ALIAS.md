# Setting Up Wilson Global Command

## ✅ What's Already Done

1. **Launcher script created:** `wilson.sh`
2. **Alias added to ~/.zshrc**

## 🔄 Activate It Now

Open a **new terminal** or run:

```bash
source ~/.zshrc
```

Then test:

```bash
# From ANY directory
wilson
```

## What Happens

When you type `wilson`, the launcher script:

1. ✅ Checks if Ollama is running
   - If not → starts it automatically
2. ✅ Checks if Wilson binary exists
   - If not → builds it automatically
3. ✅ Runs Wilson

## Verify It's Working

```bash
# Check the alias
alias | grep wilson

# Should show:
# wilson='/Users/roderick.vannievelt/IdeaProjects/wilson/wilson.sh'

# Test it
cd ~
wilson
```

## Manual Steps (if needed)

If the alias isn't working, manually add it:

```bash
# Open your .zshrc
nano ~/.zshrc

# Add this line at the end:
alias wilson='/Users/roderick.vannievelt/IdeaProjects/wilson/wilson.sh'

# Save and reload
source ~/.zshrc
```

## Troubleshooting

**"wilson: command not found"**
- Open a new terminal (aliases don't load in current session)
- Or run: `source ~/.zshrc`

**"Permission denied"**
```bash
chmod +x /Users/roderick.vannievelt/IdeaProjects/wilson/wilson.sh
```

**"Ollama failed to start"**
- Check Ollama is installed: `ollama --version`
- Try starting manually: `ollama serve`

## Uninstall

Remove the alias:
```bash
# Edit .zshrc and remove the wilson alias line
nano ~/.zshrc

# Or comment it out:
# alias wilson='/Users/roderick.vannievelt/IdeaProjects/wilson/wilson.sh'
```

---

**All set!** 🎉

Now you can just type `wilson` from anywhere to start chatting with your AI assistant.
