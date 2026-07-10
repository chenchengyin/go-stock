# Ultra-Short Trader Skills Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Trae-readable skill document and an auto-discoverable Codex skill for the user's overnight ultra-short auction trading system.

**Architecture:** Keep the trading rules as self-contained Markdown instructions in both target formats so each environment can use the skill without depending on this repository. The Trae artifact lives under the project `.trae` folder; the Codex artifact lives under the user's Codex skills directory with standard `SKILL.md` frontmatter and `agents/openai.yaml` metadata.

**Tech Stack:** Markdown, Codex skill folder structure, Trae project-local Markdown skill document.

## Global Constraints

- Preserve the user's original phrases: `该跌不跌必有大涨`, `该涨不涨必有暴跌`, `暴涨加暴涨就会调整，暴跌再暴跌就会暴涨`.
- Treat common candidate conditions as screening preferences, not absolute buy rules.
- Do not provide default position sizing advice.
- Always explain missing data and avoid deterministic profit claims.
- Create the Trae artifact inside `/Users/Zhuanz/aiproject/go-stock/.trae/skills/`.
- Install the Codex skill in `/Users/Zhuanz/.codex/skills/colin-trader/` so Codex can discover it.

---

### Task 1: Create Trae Skill Document

**Files:**
- Create: `/Users/Zhuanz/aiproject/go-stock/.trae/skills/colin_trader/SKILL.md`

**Interfaces:**
- Consumes: Approved spec at `/Users/Zhuanz/aiproject/go-stock/docs/superpowers/specs/2026-07-10-colin_trader-skill-design.md`
- Produces: A project-local Trae Markdown skill that can be referenced or attached in Trae conversations.

- [ ] **Step 1: Create `.trae/skills/colin_trader/SKILL.md`**

Write a Markdown skill with:

```markdown
# 隔夜超短竞价交易助手

用途：当用户问早盘能不能买、昨天买了今天要不要卖、某只股怎么看、盘后交易复盘时，按用户的隔夜超短交易系统输出结论和条件剧本。
```

Include these complete sections:

- Role and safety boundary.
- Two-stage answer format.
- Data acquisition rules.
- Trading time windows.
- Common candidate conditions.
- K-line strength definitions.
- Normal expectation inference.
- Core rules.
- Granville usage.
- 5/10-day risk gate.
- Market warning.
- Buy flow.
- Sell flow.
- Review learning mode.
- Answer templates.

- [ ] **Step 2: Verify Trae file content**

Run:

```bash
sed -n '1,260p' .trae/skills/colin_trader/SKILL.md
rg -n "T[O]DO|T[B]D|F[I]XME|占[位]|待[定]" .trae/skills/colin_trader/SKILL.md
```

Expected:

- `sed` shows the complete skill.
- `rg` exits with status 1 and no matches.

### Task 2: Create Codex Skill

**Files:**
- Create: `/Users/Zhuanz/.codex/skills/colin-trader/SKILL.md`
- Create: `/Users/Zhuanz/.codex/skills/colin-trader/agents/openai.yaml`

**Interfaces:**
- Consumes: Approved spec and Trae skill content.
- Produces: A Codex-discoverable skill named `colin-trader`.

- [ ] **Step 1: Initialize skill folder**

Run the skill creator initializer:

```bash
python3 /Users/Zhuanz/.codex/skills/.system/skill-creator/scripts/init_skill.py colin-trader --path /private/tmp/codex-skill-build --interface display_name="colin_trader" --interface short_description="隔夜超短竞价买卖与复盘助手" --interface default_prompt="Use $colin-trader to判断一只股票今天能不能买或昨天买了今天要不要卖。"
```

Expected: `/private/tmp/codex-skill-build/colin-trader/` exists with `SKILL.md` and `agents/openai.yaml`.

- [ ] **Step 2: Replace `SKILL.md` with final Codex instructions**

Write frontmatter exactly:

```yaml
---
name: colin-trader
description: 隔夜超短竞价交易助手。Use when the user asks whether an A-share stock can be bought in the morning auction/open, whether a position bought yesterday should be sold today, how to interpret 该跌不跌/该涨不涨, 5/10日线风险, Granville position, market warning, or wants post-trade review learning for this ultra-short system.
---
```

Then write the same operational trading rules as the Trae skill, using imperative instructions and keeping the content self-contained.

- [ ] **Step 3: Validate Codex skill build**

Run:

```bash
python3 /Users/Zhuanz/.codex/skills/.system/skill-creator/scripts/quick_validate.py /private/tmp/codex-skill-build/colin-trader
```

Expected: validation succeeds.

- [ ] **Step 4: Install Codex skill**

Run:

```bash
mkdir -p /Users/Zhuanz/.codex/skills
cp -R /private/tmp/codex-skill-build/colin-trader /Users/Zhuanz/.codex/skills/
```

Expected: `/Users/Zhuanz/.codex/skills/colin-trader/SKILL.md` exists.

### Task 3: Final Verification

**Files:**
- Verify: `/Users/Zhuanz/aiproject/go-stock/.trae/skills/colin_trader/SKILL.md`
- Verify: `/Users/Zhuanz/.codex/skills/colin-trader/SKILL.md`
- Verify: `/Users/Zhuanz/.codex/skills/colin-trader/agents/openai.yaml`

**Interfaces:**
- Consumes: Created skill artifacts.
- Produces: Verification evidence for final response.

- [ ] **Step 1: Check expected files**

Run:

```bash
test -f .trae/skills/colin_trader/SKILL.md
test -f /Users/Zhuanz/.codex/skills/colin-trader/SKILL.md
test -f /Users/Zhuanz/.codex/skills/colin-trader/agents/openai.yaml
```

Expected: all commands exit 0.

- [ ] **Step 2: Check required rule phrases**

Run:

```bash
rg -n "该跌不跌必有大涨|该涨不涨必有暴跌|暴涨加暴涨|5/10|09:50|中阳线|中阴线" .trae/skills/colin_trader/SKILL.md /Users/Zhuanz/.codex/skills/colin-trader/SKILL.md
```

Expected: matches exist in both skill files.

- [ ] **Step 3: Check no placeholders**

Run:

```bash
rg -n "T[O]DO|T[B]D|F[I]XME|占[位]|待[定]" .trae/skills/colin_trader/SKILL.md /Users/Zhuanz/.codex/skills/colin-trader/SKILL.md /Users/Zhuanz/.codex/skills/colin-trader/agents/openai.yaml
```

Expected: command exits 1 with no output.
