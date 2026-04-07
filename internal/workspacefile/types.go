package workspacefile

type Document struct {
	Source       string        `json:"source"`
	Instructions []Instruction `json:"instructions"`
}

type Instruction struct {
	Keyword string   `json:"keyword"`
	Args    []string `json:"args"`
	Line    int      `json:"line"`
	Raw     string   `json:"raw"`
}
