// What the Quest Scripts editor offers as you type, from the completions the
// server sends (quest-files/completions). Pure functions of the text before
// the cursor, so the editor only has to ask.
//
// A type here is { classes: [...] } (an object) or { listOf: [...] } (a list
// of them, from GetClientList and the like).

const PERL = "perl"

// the methods of classes and the classes they inherit from, the first
// class's first; one entry per name
export function members(data, classes) {
  const key = classes.join(",")
  data.$members = data.$members || {}
  if (data.$members[key]) {
    return data.$members[key]
  }
  const order = []
  const visit = (c) => {
    if (order.includes(c)) {
      return
    }
    order.push(c)
    ;(data.parents[c] || []).forEach(visit)
  }
  classes.forEach(visit)

  const seen = new Set()
  const out  = []
  for (const c of order) {
    for (const m of data.classes[c] || []) {
      if (!seen.has(m.name)) {
        seen.add(m.name)
        out.push(m)
      }
    }
  }
  out.sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0))
  data.$members[key] = out
  return out
}

// the type a call returns
function returnType(data, m) {
  if (!m || !m.returns) {
    return null
  }
  if (data.types[m.returns]) {
    return { classes: [data.types[m.returns]] }
  }
  // Lua lists: Lua_Client_List and the like
  const lua = m.returns.match(/Lua_(\w+?)_List\b/)
  if (lua && data.classes[lua[1]]) {
    return { listOf: [lua[1]] }
  }
  // Perl lists are arrays: GetClientList, GetNPCList, GetMobList ...
  const perl = /array/.test(m.returns) && m.name.match(/^Get(\w+?)List$/)
  if (perl && data.classes[perl[1]]) {
    return { listOf: [perl[1]] }
  }
  return null
}

// index of the ( that the ) ending s closes, skipping quoted text
function openParen(s) {
  let depth = 0
  for (let i = s.length - 1; i >= 0; i--) {
    const ch = s[i]
    if (ch === "\"" || ch === "'") {
      const j = s.lastIndexOf(ch, i - 1)
      if (j < 0) {
        return -1
      }
      i = j
      continue
    }
    if (ch === ")") {
      depth++
    } else if (ch === "(") {
      depth--
      if (depth === 0) {
        return i
      }
    }
  }
  return -1
}

export class Scope {
  // lines: the script up to the cursor; variables are tracked through them
  constructor(data, lang, lines) {
    this.data   = data
    this.lang   = lang
    this.vars   = {} // name as written ($x, x): type
    this.arrays = {} // Perl @name: type
    for (const line of lines) {
      this.track(line)
    }
  }

  track(line) {
    let m
    if (this.lang === PERL) {
      line = line.replace(/(^|\s)#.*$/, "")
      // foreach my $c ($entity_list->GetClientList()) / foreach my $c (@clients)
      if ((m = line.match(/\bfor(?:each)?\s+(?:my\s+)?\$(\w+)\s*\((.*)\)/))) {
        const list = m[2].trim().startsWith("@") ? this.arrays[m[2].trim().slice(1)] : this.typeOf(m[2])
        this.set("$" + m[1], list && list.listOf ? { classes: list.listOf } : null)
      }
      for (const a of line.matchAll(/@(\w+)\s*=(?![=~])\s*([^;]+)/g)) {
        const t = this.typeOf(a[2])
        this.arrays[a[1]] = t && t.listOf ? t : null
      }
      for (const a of line.matchAll(/\$(\w+)\s*=(?![=~>])\s*([^;]+)/g)) {
        this.set("$" + a[1], this.typeOf(a[2]))
      }
      return
    }

    line = line.replace(/--.*$/, "")
    // for i, c in pairs(list.entries) / for c in list.entries do
    if ((m = line.match(/\bfor\s+(?:\w+\s*,\s*)?(\w+)\s+in\s+(?:i?pairs\s*\()?\s*(.+?)\.entries\b/))) {
      const list = this.typeOf(m[2])
      this.set(m[1], list && list.listOf ? { classes: list.listOf } : null)
      return
    }
    if ((m = line.match(/^\s*(?:local\s+)?([A-Za-z_]\w*)\s*=(?!=)\s*(.+)$/))) {
      this.set(m[1], this.typeOf(m[2]))
    }
  }

  set(name, type) {
    if (type) {
      this.vars[name] = type
    } else {
      delete this.vars[name]
    }
  }

  // the type of the expression that ends s
  typeOf(s) {
    s = s.trim().replace(/;$/, "").trim()
    const d = this.data
    let m

    if (s.endsWith(")")) {
      const open = openParen(s)
      if (open < 0) {
        return null
      }
      return this.callType(s.slice(0, open))
    }

    if (this.lang === PERL) {
      // $client->GetTarget->  (no parentheses)
      if ((m = s.match(/->\s*(\w+)$/)) && !/\$\w+$/.test(s)) {
        return this.callType(s)
      }
      if ((m = s.match(/\$(\w+)$/))) {
        const name = "$" + m[1]
        if (this.vars[name]) {
          return this.vars[name]
        }
        return d.vars[name] ? { classes: d.vars[name] } : null
      }
      return null
    }

    if ((m = s.match(/\b(e\.\w+)$/))) {
      return this.vars[m[1]] || (d.vars[m[1]] ? { classes: d.vars[m[1]] } : null)
    }
    if ((m = s.match(/(?:^|[^.:\w])([A-Za-z_]\w*)$/))) {
      return this.vars[m[1]] || null
    }
    return null
  }

  // the type a call returns: callee is the text before its (
  callType(callee) {
    const m = this.callee(callee)
    return m ? returnType(this.data, m) : null
  }

  // the completion a callee names: quest::say, $client->Message, eq.say, e.self:Say
  callee(callee) {
    callee = callee.trimEnd()
    const d = this.data
    const re = this.lang === PERL ? /(->|::)\s*(\w+)$/ : /(:|\.)\s*(\w+)$/
    const m = callee.match(re)
    if (!m) {
      return null
    }
    const base = callee.slice(0, m.index)
    const find = (list) => (list || []).find((c) => c.name === m[2]) || null

    if (m[1] === "::" || m[1] === ".") {
      const ns = base.match(/(\w+)$/)
      if (ns && d.namespaces[ns[1] + m[1]]) {
        return find(d.namespaces[ns[1] + m[1]])
      }
      if (m[1] === "::") {
        return null
      }
    }
    const t = this.typeOf(base)
    return t && t.classes ? find(members(d, t.classes)) : null
  }

  // the innermost open call in before, and which argument the cursor is in
  callAt(before) {
    const stack = []
    let quote   = null
    for (let i = 0; i < before.length; i++) {
      const ch = before[i]
      if (quote) {
        if (ch === "\\") {
          i++
        } else if (ch === quote) {
          quote = null
        }
        continue
      }
      if (ch === "\"" || ch === "'") {
        quote = ch
      } else if (ch === "(") {
        stack.push({ open: i, arg: 0, start: i + 1 })
      } else if (ch === ")") {
        stack.pop()
      } else if (ch === "," && stack.length) {
        const top = stack[stack.length - 1]
        top.arg++
        top.start = i + 1
      }
    }
    if (quote || !stack.length) {
      return null
    }
    const top = stack[stack.length - 1]
    return { callee: before.slice(0, top.open), arg: top.arg, text: before.slice(top.start) }
  }

  // what to offer with the cursor after before: { kind, items, ... } or null
  context(before) {
    const d = this.data
    let m
    if (this.lang === PERL ? /^\s*sub\s+$/.test(before) : /^\s*function\s+$/.test(before)) {
      return { kind: "events", items: d.events }
    }
    if (this.lang === PERL) {
      if ((m = before.match(/\b(quest|plugin)::$/))) {
        return { kind: "members", key: m[1] + "::", items: d.namespaces[m[1] + "::"] || [] }
      }
      if ((m = before.match(/->\s*$/))) {
        const t = this.typeOf(before.slice(0, m.index))
        return t && t.classes ? { kind: "members", key: "->", items: members(d, t.classes) } : null
      }
    } else {
      if (/\beq\.$/.test(before)) {
        return { kind: "members", key: "eq.", items: d.namespaces["eq."] || [] }
      }
      if ((m = before.match(/:\s*$/)) && !/::\s*$/.test(before)) {
        const t = this.typeOf(before.slice(0, m.index))
        return t && t.classes ? { kind: "members", key: ":", items: members(d, t.classes) } : null
      }
    }

    // a chat type argument: $client->Message(|, quest::gmsay("hi", |
    const call = this.callAt(before)
    if (call) {
      const typed = call.text.trim()
      if (typed === "" || (this.lang !== PERL && typed === "MT.")) {
        const c = this.callee(call.callee)
        if (c && c.chat_args && c.chat_args.includes(call.arg)) {
          return { kind: "chat", call: c, arg: call.arg, typedMT: typed === "MT.", items: d.chat_types }
        }
      }
    }
    return null
  }
}

// the variables a script declares, up to the cursor, and what to offer there
export function contextAt(data, lang, linesBefore, before) {
  return new Scope(data, lang, linesBefore).context(before)
}
