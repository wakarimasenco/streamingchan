package fourchan

type Post struct {
	Board          string      `json:"board"`
	No             int         `json:"no"`
	Resto          int         `json:"resto"`
	Sticky         uint8       `json:"sticky"`
	Closed         uint8       `json:"closed"`
	Now            string      `json:"now"`
	Time           int         `json:"time"`
	Name           string      `json:"name"`
	Trip           string      `json:"trip"`
	Id             string      `json:"id"`
	Capcode        string      `json:"capcode"`
	Country        string      `json:"country"`
	CountryName    string      `json:"country_name"`
	Email          string      `json:"email"`
	Sub            string      `json:"sub"`
	Com            string      `json:"com"`
	Tim            int64       `json:"tim"`
	Filename       string      `json:"filename"`
	Md5            string      `json:"md5"`
	W              int         `json:"w"`
	H              int         `json:"h"`
	TnW            int         `json:"tn_w"`
	TnH            int         `json:"tn_h"`
	FileDeleted    uint8       `json:"filedeleted"`
	Spoiler        uint8       `json:"spoiler"`
	CustomSpoiler  int         `json:"custom_spoiler"`
	OmittedPosts   int         `json:"omitted_posts"`
	OmittedImages  int         `json:"omitted_images"`
	Replies        int         `json:"replies"`
	Images         int         `json:"images"`
	BumpLimit      uint8       `json:"bumplimit"`
	ImageLimit     uint8       `json:"imagelimit"`
	CapcodeReplies interface{} `json:"capcode_replies"`
	LastModified   int         `json:"last_modified"`
	MachineId      string      `json:"debug_machine"`
}

type Thread struct {
	Posts []Post `json:"posts"`
}

type Board struct {
	Threads []Thread `json:"threads"`
	Board   string   `json:"board"`
	Title   string   `json:"title"`
	WsBoard uint8    `json:"ws_board"`
	PerPage int      `json:"per_page"`
	Pages   int      `json:"pages"`
}

type ThreadInfo struct {
	Board        string `json:"board"`
	No           int    `json:"no"`
	LastModified int    `json:"last_modified"`
	MinPost      int    `json:"min_post"`
	OwnerId      string `json:"owner_hint"`
}

type Threads struct {
	Board   string       `json:"board"`
	Page    int          `json:"page"`
	Threads []ThreadInfo `json:"threads"`
}

type Boards struct {
	Boards []Board `json:"boards"`
}
