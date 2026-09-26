package sources

type alplakesLake struct {
	name string
	// waterBody holds another common name, so searching for it finds the lake.
	waterBody string
	lat, lon  float64
}

// alplakesLakes maps the Simstrat lake keys used by Alplakes to German names
// and the lake positions from Eawag's operational-simstrat lake parameters.
var alplakesLakes = map[string]alplakesLake{
	"aegeri":                                 {"Ägerisee", "", 47.123, 8.618},
	"amsoldingersee":                         {"Amsoldingersee", "", 46.725, 7.576},
	"baldegg":                                {"Baldeggersee", "", 47.197, 8.261},
	"biel":                                   {"Bielersee", "Lac de Bienne", 47.104, 7.198},
	"brienz":                                 {"Brienzersee", "", 46.718, 7.952},
	"champfer":                               {"Champfèrersee", "Lej da Champfèr", 46.469, 9.806},
	"geneva":                                 {"Genfersee", "Lac Léman", 46.453, 6.589},
	"greifensee":                             {"Greifensee", "", 47.35, 8.678},
	"gruyere":                                {"Greyerzersee", "Lac de la Gruyère", 46.708, 7.095},
	"hallwil":                                {"Hallwilersee", "", 47.279, 8.214},
	"joux":                                   {"Lac de Joux", "", 46.64, 6.286},
	"klontalersee":                           {"Klöntalersee", "", 47.032, 8.994},
	"lauerz":                                 {"Lauerzersee", "", 47.03, 8.607},
	"lowerconstance":                         {"Bodensee (Untersee)", "Bodensee", 47.676, 9.0},
	"lowerlugano":                            {"Luganersee (Südbecken)", "Lago di Lugano", 45.958, 8.897},
	"lowerzurich":                            {"Zürichsee", "", 47.286, 8.591},
	"lucernealpnachersee":                    {"Vierwaldstättersee (Alpnachersee)", "Lac des Quatre-Cantons", 46.968, 8.321},
	"lucernegersauerandtreibbecken":          {"Vierwaldstättersee (Gersauer- und Treibbecken)", "Lac des Quatre-Cantons", 46.979, 8.504},
	"lucernekreuztrichterandvitznauerbecken": {"Vierwaldstättersee (Kreuztrichter und Vitznauerbecken)", "Lac des Quatre-Cantons", 47.013, 8.429},
	"lucerneurnersee":                        {"Vierwaldstättersee (Urnersee)", "Lac des Quatre-Cantons", 46.953, 8.604},
	"lungern":                                {"Lungerersee", "", 46.799, 8.161},
	"maggiore":                               {"Lago Maggiore", "Langensee", 45.964, 8.645},
	"mauensee":                               {"Mauensee", "", 47.171, 8.075},
	"moossee":                                {"Moossee", "", 47.022, 7.48},
	"murten":                                 {"Murtensee", "Lac de Morat", 46.929, 7.064},
	"neuchatel":                              {"Neuenburgersee", "Lac de Neuchâtel", 46.904, 6.843},
	"oeschinensee":                           {"Oeschinensee", "", 46.498, 7.727},
	"pfaffikon":                              {"Pfäffikersee", "", 47.352, 8.784},
	"poschiavo":                              {"Lago di Poschiavo", "Puschlaversee", 46.279, 10.098},
	"rotsee":                                 {"Rotsee", "", 47.07, 8.316},
	"sarnen":                                 {"Sarnersee", "", 46.865, 8.204},
	"sempach":                                {"Sempachersee", "", 47.14, 8.159},
	"sihlsee":                                {"Sihlsee", "", 47.148, 8.782},
	"sils":                                   {"Silsersee", "Lej da Segl", 46.423, 9.739},
	"silvaplana":                             {"Silvaplanersee", "Lej da Silvaplauna", 46.449, 9.792},
	"soppensee":                              {"Soppensee", "", 47.091, 8.081},
	"stmoritz":                               {"St. Moritzersee", "Lej da San Murezzan", 46.495, 9.847},
	"thun":                                   {"Thunersee", "", 46.682, 7.733},
	"turlersee":                              {"Türlersee", "", 47.27, 8.5},
	"upperconstance":                         {"Bodensee (Obersee)", "Bodensee", 47.629, 9.374},
	"upperlugano":                            {"Luganersee (Nordbecken)", "Lago di Lugano", 46.016, 9.033},
	"upperzurich":                            {"Zürichsee (Obersee)", "", 47.205, 8.823},
	"walensee":                               {"Walensee", "", 47.123, 9.223},
	"zug":                                    {"Zugersee", "", 47.098, 8.492},
}
