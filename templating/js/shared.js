function fromMillicores(mc) {
  if (typeof (mc) == Number) {
    return mc * 1000
  }
  if (mc.substring(mc.length - 1, mc.length) == "m") {
    return Number(mc.substring(0, mc.length - 1))
  }
  return Number(mc)
}

function fromSI(si) {
  unit = si.substring(si.length - 2, si.length)
  if (unit == "Ki") {
    return Number(si.substring(0, si.length - 2)) * 1024
  }
  return Number(si)
}

function startsWith(s, search, rawPos) {
  if (s == null) {
    return false;
  }
  var pos = rawPos > 0 ? rawPos | 0 : 0;
  return s.substring(pos, pos + search.length) === search;
}

function endsWith(s, search) {
  if (s == null) {
    return false;
  }
  return s.indexOf(search) === s.length - search.length;
}
