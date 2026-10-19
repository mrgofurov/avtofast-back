package domain

// The syllabus as topics. Each topic is a chapter of the Yo'l harakati
// qoidalari, a section of one of its appendices, or (for the few subjects the
// Rules do not cover) a subject of its own. Following the Rules' own
// structure keeps topics recognisable to a learner who has the Rules open, and
// lets a question be filed by the clause its official explanation cites
// rather than by guesswork. Questions store the topic key in their category.

// TopicGroup is a heading on the topics screen.
type TopicGroup struct {
	Key   string `json:"key"`
	Title string `json:"title"`
}

// Topic is one entry of the syllabus.
type Topic struct {
	Key   string
	Group string
	Title string
	// Source names what the topic covers in the Rules, for the learner:
	// "YHQ 16-bob, 105, 107–108-bandlar". Empty for topics outside them.
	Source string

	// Where the Rules put it. Chapter > 0 for a chapter (Bands narrows it to
	// some of the chapter's bands); Appendix and Section for an appendix
	// section (Section 0: the whole appendix).
	Chapter  int
	Bands    []int
	Appendix int
	Section  int
}

const (
	TopicGroupSigns        = "signs"
	TopicGroupIntersection = "intersections"
	TopicGroupSignals      = "signals"
	TopicGroupManeuvers    = "maneuvers"
	TopicGroupSpecial      = "special"
	TopicGroupGeneral      = "general"
	TopicGroupTransport    = "transport"
	TopicGroupSafety       = "safety"
)

var TopicGroups = []TopicGroup{
	{TopicGroupSigns, "Yoʻl belgilari"},
	{TopicGroupIntersection, "Chorrahalar"},
	{TopicGroupSignals, "Ishoralar va chiziqlar"},
	{TopicGroupManeuvers, "Asosiy manyovrlar"},
	{TopicGroupSpecial, "Maxsus vaziyatlar"},
	{TopicGroupGeneral, "Umumiy qoidalar"},
	{TopicGroupTransport, "Tashish va qoʻshimcha talablar"},
	{TopicGroupSafety, "Xavfsizlik va texnik holat"},
}

// Topic keys for the subjects that have no chapter of their own.
const (
	TopicFirstAid    = "first_aid"
	TopicSafeDriving = "safe_driving"
	TopicLiability   = "liability"
)

// Topics is the syllabus in the order the app lists it.
var Topics = []Topic{
	{Key: "signs_warning", Group: TopicGroupSigns, Title: "Ogohlantiruvchi belgilar", Source: "YHQ 1-ilova, 1-boʻlim", Appendix: 1, Section: 1},
	{Key: "signs_priority", Group: TopicGroupSigns, Title: "Imtiyoz belgilari", Source: "YHQ 1-ilova, 2-boʻlim", Appendix: 1, Section: 2},
	{Key: "signs_prohibitory", Group: TopicGroupSigns, Title: "Taqiqlovchi belgilar", Source: "YHQ 1-ilova, 3-boʻlim", Appendix: 1, Section: 3},
	{Key: "signs_mandatory", Group: TopicGroupSigns, Title: "Buyuruvchi belgilar", Source: "YHQ 1-ilova, 4-boʻlim", Appendix: 1, Section: 4},
	{Key: "signs_information", Group: TopicGroupSigns, Title: "Axborot-koʻrsatgich belgilari", Source: "YHQ 1-ilova, 5-boʻlim", Appendix: 1, Section: 5},
	{Key: "signs_service", Group: TopicGroupSigns, Title: "Servis belgilari", Source: "YHQ 1-ilova, 6-boʻlim", Appendix: 1, Section: 6},
	{Key: "signs_additional", Group: TopicGroupSigns, Title: "Qoʻshimcha axborot belgilari", Source: "YHQ 1-ilova, 7-boʻlim", Appendix: 1, Section: 7},

	{Key: "intersections_general", Group: TopicGroupIntersection, Title: "Chorrahalarda harakatlanish", Source: "YHQ 14-bob", Chapter: 14},
	{Key: "intersections_regulated", Group: TopicGroupIntersection, Title: "Tartibga solingan chorrahalar", Source: "YHQ 15-bob", Chapter: 15},
	{Key: "intersections_equal", Group: TopicGroupIntersection, Title: "Tartibga solinmagan teng ahamiyatli chorrahalar", Source: "YHQ 16-bob, 105, 107–108-bandlar", Chapter: 16, Bands: []int{105, 107, 108}},
	{Key: "intersections_main_road", Group: TopicGroupIntersection, Title: "Tartibga solinmagan chorrahalar: asosiy yoʻl", Source: "YHQ 16-bob, 104-band", Chapter: 16, Bands: []int{104}},
	{Key: "intersections_main_road_turns", Group: TopicGroupIntersection, Title: "Asosiy yoʻlning yoʻnalishi oʻzgaradigan chorrahalar", Source: "YHQ 16-bob, 106-band", Chapter: 16, Bands: []int{106}},

	{Key: "signals_traffic_lights", Group: TopicGroupSignals, Title: "Svetofor ishoralari", Source: "YHQ 7-bob, 30–37-bandlar", Chapter: 7, Bands: []int{30, 31, 32, 33, 34, 35, 36, 37}},
	{Key: "signals_regulator", Group: TopicGroupSignals, Title: "Tartibga soluvchining ishoralari", Source: "YHQ 7-bob, 38–43-bandlar", Chapter: 7, Bands: []int{38, 39, 40, 41, 42, 43}},
	{Key: "signals_warning", Group: TopicGroupSignals, Title: "Ogohlantiruvchi va avariya ishoralari", Source: "YHQ 8-bob", Chapter: 8},
	{Key: "markings_horizontal", Group: TopicGroupSignals, Title: "Yotiq chiziqlar", Source: "YHQ 2-ilova, 1-boʻlim", Appendix: 2, Section: 1},
	{Key: "markings_vertical", Group: TopicGroupSignals, Title: "Tik chiziqlar", Source: "YHQ 2-ilova, 2-boʻlim", Appendix: 2, Section: 2},

	{Key: "maneuvering", Group: TopicGroupManeuvers, Title: "Harakatlanishni boshlash, manyovr qilish", Source: "YHQ 9-bob", Chapter: 9},
	{Key: "lane_position", Group: TopicGroupManeuvers, Title: "Qatnov qismida transport vositalarining joylashuvi", Source: "YHQ 10-bob", Chapter: 10},
	{Key: "speed", Group: TopicGroupManeuvers, Title: "Harakatlanish tezligi", Source: "YHQ 11-bob", Chapter: 11},
	{Key: "overtaking", Group: TopicGroupManeuvers, Title: "Quvib oʻtish", Source: "YHQ 12-bob", Chapter: 12},
	{Key: "stopping_parking", Group: TopicGroupManeuvers, Title: "Toʻxtash va toʻxtab turish", Source: "YHQ 13-bob", Chapter: 13},

	{Key: "pedestrian_crossings", Group: TopicGroupSpecial, Title: "Piyodalar oʻtish joylari va bekatlar", Source: "YHQ 17-bob", Chapter: 17},
	{Key: "railway_crossings", Group: TopicGroupSpecial, Title: "Temir yoʻl kesishmalari", Source: "YHQ 18-bob", Chapter: 18},
	{Key: "motorways", Group: TopicGroupSpecial, Title: "Avtomagistrallarda harakatlanish", Source: "YHQ 19-bob", Chapter: 19},
	{Key: "residential_areas", Group: TopicGroupSpecial, Title: "Turar joy dahalarida harakatlanish", Source: "YHQ 20-bob", Chapter: 20},
	{Key: "slopes", Group: TopicGroupSpecial, Title: "Tik balandlik va nishabliklarda harakatlanish", Source: "YHQ 21-bob", Chapter: 21},
	{Key: "route_vehicles", Group: TopicGroupSpecial, Title: "Yoʻnalishli transport vositalarining imtiyozlari", Source: "YHQ 22-bob", Chapter: 22},
	{Key: "special_vehicles", Group: TopicGroupSpecial, Title: "Maxsus transport vositalarining imtiyozlari", Source: "YHQ 6-bob", Chapter: 6},
	{Key: "lighting", Group: TopicGroupSpecial, Title: "Tashqi yoritish asboblaridan foydalanish", Source: "YHQ 23-bob", Chapter: 23},

	{Key: "general_rules", Group: TopicGroupGeneral, Title: "Umumiy qoidalar va asosiy tushunchalar", Source: "YHQ 1-bob", Chapter: 1},
	{Key: "driver_duties", Group: TopicGroupGeneral, Title: "Haydovchilarning umumiy vazifalari", Source: "YHQ 2-bob", Chapter: 2},
	{Key: "accidents", Group: TopicGroupGeneral, Title: "Yoʻl-transport hodisasida haydovchining majburiyatlari", Source: "YHQ 3-bob", Chapter: 3},
	{Key: "pedestrians", Group: TopicGroupGeneral, Title: "Piyodalarning majburiyatlari", Source: "YHQ 4-bob", Chapter: 4},
	{Key: "passengers", Group: TopicGroupGeneral, Title: "Yoʻlovchilarning majburiyatlari", Source: "YHQ 5-bob", Chapter: 5},
	{Key: TopicLiability, Group: TopicGroupGeneral, Title: "Haydovchining huquqiy javobgarligi", Source: "MJtK, Yoʻl harakati toʻgʻrisidagi qonun"},

	{Key: "towing", Group: TopicGroupTransport, Title: "Mexanik transport vositalarini shatakka olish", Source: "YHQ 24-bob", Chapter: 24},
	{Key: "driving_training", Group: TopicGroupTransport, Title: "Transport vositalarini boshqarishni oʻrgatish", Source: "YHQ 25-bob", Chapter: 25},
	{Key: "passenger_transport", Group: TopicGroupTransport, Title: "Odam tashish", Source: "YHQ 26-bob", Chapter: 26},
	{Key: "cargo_transport", Group: TopicGroupTransport, Title: "Yuk tashish", Source: "YHQ 27-bob", Chapter: 27},
	{Key: "bicycles_mopeds", Group: TopicGroupTransport, Title: "Velosiped, moped va aravalar, hayvonlarni haydab oʻtish", Source: "YHQ 28-bob", Chapter: 28},

	{Key: TopicFirstAid, Group: TopicGroupSafety, Title: "Birinchi tibbiy yordam", Source: ""},
	{Key: TopicSafeDriving, Group: TopicGroupSafety, Title: "Harakat xavfsizligi asoslari", Source: ""},
	{Key: "vehicle_faults", Group: TopicGroupSafety, Title: "Transport vositalaridan foydalanishni taqiqlovchi shartlar", Source: "YHQ 3-ilova", Appendix: 3},
	{Key: "registration_marks", Group: TopicGroupSafety, Title: "Yoʻlga chiqarish talablari va taniqli belgilar", Source: "YHQ 29-bob", Chapter: 29},
	{Key: "danger_plates", Group: TopicGroupSafety, Title: "Xavflilik belgilari", Source: "YHQ 4-ilova", Appendix: 4},
}

// Categories is the syllabus's topic keys, in the order the app lists them.
var Categories = func() []string {
	keys := make([]string, len(Topics))
	for i, t := range Topics {
		keys[i] = t.Key
	}
	return keys
}()

// IsKnownCategory reports whether a category name is one of the topics. Used
// to turn an unrecognised path parameter into a 400 rather than an empty list
// that looks like a topic with no questions in it.
func IsKnownCategory(category string) bool {
	_, ok := TopicByKey(category)
	return ok
}

func TopicByKey(key string) (Topic, bool) {
	for _, t := range Topics {
		if t.Key == key {
			return t, true
		}
	}
	return Topic{}, false
}

func TopicGroupTitle(key string) string {
	for _, g := range TopicGroups {
		if g.Key == key {
			return g.Title
		}
	}
	return ""
}
