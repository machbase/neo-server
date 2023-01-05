package shell

func (sess *Session) exec_explain(line string) {
	sess.log.Debugf("EXPLAIN: %s", line)
	plan, err := sess.db.Explain(line)
	if err != nil {
		sess.WriteStr(err.Error() + "\r\n")
		return
	}
	sess.WriteStr(plan)
}
