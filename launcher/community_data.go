package main

// Made-up community uploads, so the browser has something in it until there's a real server.

func cst(id, name, author string, min, max int, building, loadout string, tags []string, story, tip string, dl, likes, days int, roles []Role, comments ...CommunityComment) CommunityStory {
	return CommunityStory{
		Backstory: Backstory{ID: "c-" + id, Name: name, Min: min, Max: max, Building: building, Loadout: loadout, Tags: tags, Story: story, Tip: tip, Roles: roles},
		Author:    author, Downloads: dl, Likes: likes, Days: days, Comments: comments,
	}
}

func cm(author string, days int, text string) CommunityComment {
	return CommunityComment{Author: author, Text: text, Days: days}
}

func tg(t ...string) []string { return t }

var communityStories = []CommunityStory{
	cst("wedding", "The Wedding", "Bunker_Brad", 3, 8, "church", "partypack", tg("town", "easy"),
		"The bride's uncle looked sick at the rehearsal. Nobody said anything. Now he's eating the flower girl's dad.",
		"Church start in a suit. Good for big groups.", 48210, 3912, 41,
		[]Role{r("Bride", "unemployed", "It was supposed to be her day.", "brave", "athletic"), r("Groom", "constructionworker", "Cold feet, now cold everything.", "strong", "cowardly"), r("Best Man", "unemployed", "Lost the rings. Found a crowbar.", "brave", "smoker"), r("Maid of Honor", "nurse", "Kept everyone calm. Mostly.", "organized"), r("Drunk Uncle", "unemployed", "Was already slurring before it started.", "stout", "clumsy"), r("Pastor", "unemployed", "Has seen some things.", "keenhearing", "unfit"), r("Caterer", "chef", "Brought knives. Lots of knives.", "cook", "fastlearner"), r("Photographer", "unemployed", "Still filming.", "eagleeyed", "weak")},
		cm("rotten_ryan", 12, "did this with 7 of us and the drunk uncle died in the first 4 minutes. 10/10"), cm("Kayla_KY", 30, "the suits are actually decent armor lol"), cm("Mortimer", 39, "needs a cake item")),
	cst("pizza", "30 Minutes or Less", "SpiffoFan88", 1, 3, "store", "basics", tg("town"),
		"Last delivery of the night. The customer didn't answer the door. Then they did.",
		"Store start, short on food. Get moving.", 21877, 1730, 18,
		[]Role{r("Driver", "burgerflipper", "Knows every back road.", "speeddemon", "fastlearner"), r("Cook", "chef", "Stayed at the shop.", "cook"), r("Manager", "unemployed", "Wants the van back.", "organized", "unfit")},
		cm("Pete_Plays", 3, "speed demon driver is busted for looting runs")),
	cst("bunker", "Bunker Dad", "Bunker_Brad", 1, 4, "farm", "prepper", tg("easy", "wild", "long"),
		"Dad built a bunker in 1983. Everyone laughed. Nobody's laughing now. Dad is, a bit.",
		"Farm start with a prepper kit. Easy mode honestly.", 33490, 2205, 77,
		[]Role{r("Dad", "veteran", "Told you so.", "organized", "hardofhearing"), r("Mom", "nurse", "Rationing everything.", "firstaid", "lighteater"), r("Teen", "unemployed", "No wifi. Furious.", "jogger", "fastlearner", "slowreader"), r("Grandma", "unemployed", "Seen worse.", "brave", "weak")},
		cm("NotABot_Dave", 60, "way too easy with the prepper kit, swap it for basics"), cm("Bunker_Brad", 59, "that's the point dave")),
	cst("roadtrip", "Spring Break '93", "kentucky_kid", 2, 5, "motel", "partypack", tg("town"),
		"Five college kids, one car, no plan. The motel had a pool and a body in it.",
		"Motel start, party kit. Find a car fast.", 18660, 1301, 25,
		[]Role{r("Driver", "unemployed", "Owns the car. Won't let you forget it.", "speeddemon"), r("Jock", "fitnessinstructor", "Brought the beer.", "athletic", "slowlearner"), r("Pre-Med", "unemployed", "Two years of biology.", "firstaid", "cowardly"), r("Stoner", "unemployed", "Not sure this is real.", "lowthirst", "slowreader"), r("Tagalong", "unemployed", "Someone's little brother.", "jogger", "weak")}),
	cst("ghosttown", "Ghost Town Sheriff", "OldManJenkins", 1, 2, "police", "rifles", tg("hard"),
		"You're the only law in a town of 400. Was 400.",
		"Small-town cop with a rifle. Play it slow.", 9120, 733, 102,
		[]Role{r("Sheriff", "policeofficer", "Been here 30 years.", "eagleeyed", "unfit"), r("Deputy", "securityguard", "Second week on the job.", "jogger", "cowardly")}),
	cst("icecream", "Ice Cream Truck", "Kayla_KY", 1, 2, "store", "basics", tg("town", "easy"),
		"The jingle still plays. Everything in town hears it.",
		"Store start. You'll want to find the truck.", 12403, 1180, 9,
		[]Role{r("Driver", "unemployed", "Has the keys.", "sundaydriver", "inconspicuous"), r("Helper", "unemployed", "Was handing out cones.", "fastlearner")},
		cm("SpiffoFan88", 2, "is there an actual truck or nah"), cm("Kayla_KY", 2, "go find one :)")),
	cst("nursinghome", "Bingo Night", "rotten_ryan", 2, 6, "hospital", "medics", tg("hard", "town"),
		"Tuesday bingo at the care home. B-12 bit G-50.",
		"Old folks run. Everyone slow, nobody fast.", 15802, 1544, 33,
		[]Role{r("Nurse", "nurse", "Twenty years of night shifts.", "firstaid", "needslesssleep"), r("Resident", "unemployed", "Ninety and furious.", "brave", "unfit", "hardofhearing"), r("Resident", "unemployed", "Still sharp as a tack.", "fastreader", "weak"), r("Cook", "cook", "Made the meatloaf.", "cook"), r("Groundskeeper", "unemployed", "Has a shovel.", "strong", "smoker"), r("Grandkid", "unemployed", "Was just visiting.", "jogger")}),
	cst("survivalist", "Naked and Afraid", "OldManJenkins", 1, 2, "cabin", "none", tg("hard", "wild"),
		"The TV show dropped you in the woods with nothing. The crew never came back.",
		"Woods start with nothing at all.", 26004, 2450, 58,
		[]Role{r("Contestant", "unemployed", "Knows a bit about plants.", "herbalist", "outdoorsman", "weak"), r("Contestant", "unemployed", "Signed up for the money.", "athletic", "cowardly")},
		cm("NotABot_Dave", 50, "died to a log. then to a zombie."), cm("Mortimer", 41, "good with the clothes turned off in sandbox")),
	cst("frat", "Frat House", "Pete_Plays", 3, 8, "bighouse", "partypack", tg("town"),
		"Rush week. Pledges were told to guard the door no matter what. They took it seriously.",
		"Big house, lots of beer, lots of idiots.", 30551, 2012, 21,
		[]Role{r("President", "unemployed", "Runs the house.", "organized", "brave"), r("Pledge", "unemployed", "Still wearing the hazing outfit.", "jogger", "slowlearner"), r("Pledge", "unemployed", "Doesn't want to be here.", "cowardly", "fastlearner"), r("Treasurer", "unemployed", "Has the beer money.", "inconspicuous"), r("Athlete", "fitnessinstructor", "On scholarship.", "athletic", "illiterate"), r("Neighbour", "unemployed", "Came to complain about the noise.", "keenhearing", "unfit")}),
	cst("trucker", "Convoy", "BigRigRandy", 2, 6, "gas", "mechanics", tg("town"),
		"Six trucks, one CB channel, and a highway full of dead.",
		"Gas station start with tools. Cars are the plan.", 11330, 845, 64,
		[]Role{r("Lead Truck", "mechanics", "Calls the shots on the CB.", "mechanics", "smoker"), r("Rookie", "unemployed", "First run.", "fastlearner", "sundaydriver"), r("Mechanic", "repairman", "Keeps them running.", "handy"), r("Night Driver", "unemployed", "Never sleeps.", "needslesssleep", "unfit")}),
	cst("lighthouse", "Island Keepers", "Mortimer", 1, 3, "cabin", "fishers", tg("wild", "long", "easy"),
		"The ferry stopped running. You've got fish and a long winter ahead.",
		"Lake cabin start with fishing gear.", 8754, 690, 120,
		[]Role{r("Keeper", "fisherman", "Lives here year round.", "fishing", "agoraphobic"), r("Assistant", "unemployed", "Summer job.", "jogger"), r("Visitor", "unemployed", "Missed the last ferry.", "cowardly", "fastreader")}),
	cst("cult", "The Compound", "rotten_ryan", 3, 8, "farm", "melee", tg("hard", "wild"),
		"The leader said the end was coming. He was right, which is the worst part.",
		"Farm start, melee only. Everyone has a weird name.", 19032, 1688, 14,
		[]Role{r("Leader", "unemployed", "Has a plan. Probably.", "brave", "conspicuous"), r("Right Hand", "securityguard", "Does the dirty work.", "strong", "illiterate"), r("Farmer", "farmer", "Feeds everyone.", "gardener"), r("Doubter", "unemployed", "Wants out.", "inconspicuous", "cowardly"), r("Medic", "nurse", "Herbs only.", "herbalist"), r("New Member", "unemployed", "Joined last week.", "fastlearner", "weak")}),
	cst("hockey", "Away Game", "kentucky_kid", 3, 8, "school", "melee", tg("town"),
		"Team bus broke down outside a school. At least you brought the sticks.",
		"School start, melee kit. Team sport.", 14580, 1022, 37,
		[]Role{r("Coach", "fitnessinstructor", "Yells a lot.", "brave", "unfit"), r("Captain", "baseballplayer", "Leads from the front.", "athletic"), r("Goalie", "unemployed", "Big and slow.", "strong", "stout"), r("Benchwarmer", "unemployed", "Finally gets to play.", "jogger", "cowardly"), r("Bus Driver", "unemployed", "Not getting paid enough.", "sundaydriver")}),
	cst("bank", "Heist Gone Wrong", "Pete_Plays", 2, 5, "store", "armed", tg("town", "hard"),
		"You robbed the bank on the worst possible day. The cops outside aren't cops anymore.",
		"Store start, armed. Loud and messy.", 40112, 3301, 47,
		[]Role{r("Planner", "unemployed", "Had it all worked out.", "organized", "smoker"), r("Muscle", "securityguard", "Doesn't talk much.", "strong"), r("Driver", "unemployed", "Getaway car is two blocks away.", "speeddemon"), r("Safecracker", "burglar", "Steady hands.", "dextrous", "nervouswreck"), r("Hostage", "unemployed", "Just wanted to deposit a check.", "inconspicuous", "cowardly")},
		cm("BigRigRandy", 40, "hostage is the best role, no one suspects you"), cm("Kayla_KY", 22, "we left the hostage behind on day 1")),
	cst("scouts", "Troop 42", "Mortimer", 2, 6, "camp", "campers", tg("wild", "easy"),
		"The scoutmaster went to get help two days ago.",
		"Summer camp start. Badges matter now.", 7311, 512, 88,
		[]Role{r("Scoutmaster", "parkranger", "Actually did come back.", "outdoorsman", "unfit"), r("Eagle Scout", "unemployed", "Has every badge.", "hiker", "fastlearner"), r("Troublemaker", "unemployed", "Set the last fire.", "inconspicuous"), r("Homesick", "unemployed", "Wants mom.", "cowardly", "keenhearing")}),
	cst("rv", "Snowbirds", "BigRigRandy", 2, 2, "trailer", "basics", tg("easy"),
		"Retired, heading south in the RV. Took a wrong exit into Kentucky.",
		"Trailer start, two old timers.", 6120, 470, 70,
		[]Role{r("Husband", "unemployed", "Won't ask for directions.", "handy", "hardofhearing"), r("Wife", "unemployed", "Has the map.", "organized", "shortsighted")}),
	cst("vet", "Animal Shelter", "Kayla_KY", 1, 4, "warehouse", "medics", tg("town"),
		"You stayed to feed the dogs. They're the only ones who didn't turn.",
		"Warehouse start with medical stuff.", 9876, 990, 16,
		[]Role{r("Vet", "doctor", "Used to stitching.", "firstaid"), r("Volunteer", "unemployed", "Here on weekends.", "jogger"), r("Dog Walker", "unemployed", "Fast on her feet.", "athletic", "weak")}),
	cst("radio", "Last Broadcast", "Mortimer", 1, 3, "warehouse", "electric", tg("town", "long"),
		"Somebody has to keep the station on air. Somebody might be listening.",
		"Warehouse start, electrical gear.", 5402, 433, 140,
		[]Role{r("DJ", "unemployed", "The voice of Knox County.", "fastreader", "unfit"), r("Engineer", "electrician", "Keeps the signal up.", "handy"), r("Intern", "unemployed", "Gets the coffee.", "jogger")}),
	cst("mallcops", "Mall Cops", "SpiffoFan88", 2, 5, "mall", "none", tg("town", "hard"),
		"Paid minimum wage to guard the mall. Still guarding it.",
		"Mall start with nothing.", 22344, 1702, 29,
		[]Role{r("Head of Security", "securityguard", "Takes it way too seriously.", "brave", "unfit"), r("Night Guard", "securityguard", "Mostly naps.", "needslesssleep"), r("Janitor", "unemployed", "Has every key.", "inconspicuous", "handy"), r("Food Court", "burgerflipper", "Free pretzels for life.", "cook")}),
	cst("hunting", "Deer Camp", "OldManJenkins", 2, 4, "cabin", "hunters", tg("wild"),
		"Same cabin, same week, every year since '71.",
		"Cabin start with shotguns.", 13775, 990, 95,
		[]Role{r("Grandpa", "unemployed", "Still the best shot.", "eagleeyed", "weak"), r("Dad", "hunter", "Tells the same stories.", "outdoorsman"), r("Son", "unemployed", "First trip.", "fastlearner", "cowardly")}),
	cst("garage", "Garage Band", "kentucky_kid", 3, 5, "house", "partypack", tg("town", "easy"),
		"You were finally getting good. Then the neighbours came over. All of them.",
		"House start. Noise attracts. Be smart.", 10233, 880, 11,
		[]Role{r("Singer", "unemployed", "Loud.", "conspicuous", "brave"), r("Guitar", "unemployed", "Wants to go solo.", "dextrous"), r("Drums", "unemployed", "Has sticks. Hits things.", "strong", "hardofhearing"), r("Bass", "unemployed", "Nobody notices him.", "inconspicuous")}),
	cst("jail", "Drunk Tank", "rotten_ryan", 2, 6, "police", "none", tg("hard", "town"),
		"Woke up in a cell. Hungover. The officer with the keys is chewing on the bars.",
		"Police station start with nothing.", 17410, 1430, 52,
		[]Role{r("Drunk", "unemployed", "Doesn't remember last night.", "stout", "clumsy"), r("Pickpocket", "burglar", "Lifted the keys once before.", "dextrous", "inconspicuous"), r("Bar Fighter", "unemployed", "Won the fight.", "strong", "illiterate"), r("Officer", "policeofficer", "Locked in with them.", "brave")}),
	cst("doctor", "Plague Doctors", "NotABot_Dave", 2, 4, "hospital", "medics", tg("hard", "long"),
		"The CDC sent you to Knox County to study the outbreak. Your ride left without you.",
		"Hospital start. Focus on cures, not fights.", 6650, 512, 33,
		[]Role{r("Lead Scientist", "doctor", "Thinks there's a cure.", "fastreader", "weak"), r("Field Medic", "nurse", "Seen it up close.", "firstaid", "brave"), r("Guard", "securityguard", "Assigned to protect you.", "strong")}),
	cst("lumber", "Mill Town", "BigRigRandy", 2, 6, "warehouse", "lumberjacks", tg("wild", "long"),
		"The mill blew its whistle at noon. Nobody came back from lunch.",
		"Warehouse start with axes.", 8890, 645, 66,
		[]Role{r("Foreman", "lumberjack", "Runs the floor.", "strong", "smoker"), r("Sawyer", "carpenter", "Missing two fingers.", "handy"), r("Driver", "mechanics", "Hauls the logs.", "mechanics"), r("Office", "unemployed", "Does the payroll.", "organized", "unfit")}),
	cst("kids", "Home Alone", "Pete_Plays", 1, 3, "bighouse", "basics", tg("easy", "town"),
		"The family flew to Paris. You're eight. You have a plan.",
		"Big house start. Booby traps encouraged.", 36500, 3880, 5,
		[]Role{r("Kid", "unemployed", "Eight years old and ready.", "inconspicuous", "fastlearner", "weak"), r("Older Brother", "unemployed", "Came back for you.", "athletic"), r("Neighbour", "unemployed", "Shovels snow. Seems scary.", "strong", "agoraphobic")},
		cm("Kayla_KY", 1, "paint cans off the stairs when"), cm("SpiffoFan88", 4, "this is the one")),
	cst("blackout", "Power Plant", "NotABot_Dave", 2, 5, "warehouse", "electric", tg("hard", "long"),
		"Keep the plant running and the town has lights. Stop, and it's dark forever.",
		"Warehouse start. Generators matter.", 5100, 380, 110,
		[]Role{r("Shift Lead", "engineer", "Knows the turbines.", "handy", "fastlearner"), r("Electrician", "electrician", "Fixes everything.", "organized"), r("Guard", "securityguard", "Watches the fence.", "keenhearing"), r("Cleaner", "unemployed", "Wrong place, wrong time.", "jogger")}),
	cst("church2", "Choir Practice", "Mortimer", 3, 8, "church", "basics", tg("town", "easy"),
		"Thursday choir. The alto section turned first.",
		"Church start. Good if half your friends are new.", 4410, 360, 150,
		[]Role{r("Director", "unemployed", "Keeps time.", "organized"), r("Soprano", "unemployed", "Loud.", "conspicuous"), r("Tenor", "unemployed", "Took the bus.", "jogger"), r("Bass", "unemployed", "Big guy.", "strong", "unfit"), r("Organist", "unemployed", "Doesn't hear much.", "dextrous", "hardofhearing")}),
	cst("farm2", "Moonshiners", "OldManJenkins", 2, 4, "farm", "farmers", tg("wild", "long"),
		"Grandpa's still has been running since prohibition. The revenue man is dead. So is everyone else.",
		"Farm start. Long game.", 11200, 970, 44,
		[]Role{r("Brewer", "farmer", "Knows the recipe.", "gardener", "smoker"), r("Runner", "unemployed", "Fastest car in the county.", "speeddemon"), r("Lookout", "unemployed", "Always watching the road.", "eagleeyed", "cowardly"), r("Cousin", "unemployed", "Mostly drinks it.", "stout", "clumsy")}),
}

func pm(id, name, ws string) CommunityMod { return CommunityMod{ID: id, Name: name, WS: ws} }

// Some mods use their real Workshop ids; the rest are known Workshop mods with no id
// here, so the browser offers a Workshop search for them.
var (
	cmZB         = pm("ZombieBuddy", "ZombieBuddy", "3619862853")
	cmViewpoint  = pm("Viewpoint", "Viewpoint", "3809306528")
	cmVPDoor     = pm("ViewpointDoor", "Viewpoint - Door Fix", "3812151347")
	cmVPLight    = pm("ViewpointLight", "Viewpoint - Light Fix", "3811344119")
	cmVPSurface  = pm("ViewpointSurface", "Viewpoint - Surface Fix", "3814398576")
	cmVPRight    = pm("ViewpointRightClick", "Viewpoint - Right Click Fix", "3819018807")
	cmVPMove     = pm("ViewpointAdvancedMovement", "Viewpoint Advanced Movement", "3812702301")
	cmVPBlood    = pm("ViewpointBloodFX", "Blood FX for Viewpoint", "3811472160")
	cmVPWeather  = pm("ViewpointTrueWeathers", "Project Viewpoint: True Weathers & Lighting", "3813908957")
	cmVPFences   = pm("ViewpointFences3D", "Viewpoint - 3D Fences", "3814486926")
	cmVPDoors3D  = pm("BasileDoors3D", "Project Viewpoint - 3D Doors, Gates & Garages", "3818859807")
	cmVPCurtains = pm("DGVCurtains", "3D Curtains & Blinds for Viewpoint", "3818873810")
	cmVPWindows  = pm("DGVWindows3D", "3D Windows for Viewpoint", "3820833348")
	cmVPTrees    = pm("ProjectViewpointNearVegetation", "Project Viewpoint: TREE in 3D", "3813172420")
	cmVPStars    = pm("ProjectViewpointStars", "Project Viewpoint: Kentucky Nights", "3821241473")
	cmVPRoofs    = pm("ViewpointRoofs", "Viewpoint - Roofs (WIP)", "3817294484")
	cmVPADS      = pm("ProjectViewpointADS", "Project Viewpoint: Recoil & ADS", "3812612054")
	cmVPBallist  = pm("ViewpointTrueBallistics", "Viewpoint True Ballistics", "3812864848")
	cmVPFreeAim  = pm("ViewpointFreeAim", "Viewpoint Free Aim", "3820063546")
	cmVPTactical = pm("ViewpointTactical", "Viewpoint Tactical - 3D Weapons", "3819385365")
	cmVPHeli     = pm("HeliEvent", "3D Helicopter Event - Project Viewpoint", "3818359767")
	cmVPSize     = pm("ViewpointSizeMatters", "Viewpoint Size Matters", "3821185516")
	cmVPDrive    = pm("ImmersiveDriving", "Immersive Driving", "3817413313")
	cmVPSnow     = pm("ImmersiveSnow", "Immersive Snow", "3810071828")
	cmVPRagdoll  = pm("ProperRagdolls", "Proper Ragdolls", "3821280085")
	cmVisceral   = pm("VisceralViewpoint", "Visceral (Viewpoint)", "3820965001")
	cmVoxel      = pm("PZVoxelStudioViewpoint", "6277 3D models for Viewpoint", "3810302175")
	cmTrue3D     = pm("ProjectFullHD", "True 3D Items", "3803108123")
	cmALife      = pm("ProjectALifeNPCs", "Project A-Life", "3803984183")
	cmALifeJ     = pm("ProjectALifeJimmy", "Project A-Life - Jeem Extension", "3806944055")
	cmALifeVP    = pm("ViewpointALife", "Viewpoint x A-LIFE", "3822854313")
	cmXannyCore  = pm("XannyArmsCore", "Xanny Arms Core", "3820971639")
	cmXannyGuns  = pm("XannyArmsGunPack", "Xanny Arms GunPack", "3820971639")
	cmXannyVP    = pm("XannyArmsViewpoint", "Xanny Arms - Viewpoint Compatibility", "3820972331")
	cmGunRacks   = pm("Tuna_GunRacks", "3D Gun Racks", "3784677588")
	cmLockers    = pm("Tuna_EquipmentRacks", "3D Equipment Lockers", "3793403177")
	cmRV         = pm("DeadWorksNomadRV", "Nomad RV", "3820158673")
	cmVans       = pm("GridVans", "Viewpoint - 3D RV Van", "3821174725")
	cmHair       = pm("improvedhairmenubuild42", "Improved Hair Menu Build 42", "3589548354")
	cmSpnHair    = pm("SpnHair", "Spongie's Hair", "2463184726")
	cmFluffy     = pm("FH", "Fluffy Hair", "2447729538")
	cmYaki       = pm("YakiHRB42", "Yaki's Hair Retexture - B42", "2787539183")
	cmSpiffBoy   = pm("SpiffBoy", "SpiffBoy", "3825220364")

	cmCommonSense = pm("CommonSense", "Common Sense", "")
	cmSkillJrnl   = pm("SkillRecoveryJournal", "Skill Recovery Journal", "")
	cmTrueMusic   = pm("truemusic", "True Music", "")
	cmAuthZ       = pm("AuthenticZCurrent", "Authentic Z", "")
	cmBandits     = pm("Bandits", "Bandits", "")
	cmWeekOne     = pm("WeekOne", "Week One", "")
	cmEHE         = pm("ExpandedHelicopterEvents", "Expanded Helicopter Events", "")
	cmRavenCreek  = pm("RavenCreek", "Raven Creek", "")
	cmEerie       = pm("EerieCountry", "Eerie Country", "")
	cmBetterSort  = pm("BetterSorting", "Better Sorting", "")
	cmMoreTraits  = pm("ToadTraits", "More Traits", "")
	cmDynTraits   = pm("DynamicTraits", "Dynamic Traits", "")
	cmFilibuster  = pm("FRUsedCars", "Filibuster Rhymes' Used Cars", "")
	cmAutotsar    = pm("autotsartrailers", "Autotsar Trailers", "")
	cmBrita       = pm("Brita", "Brita's Weapon Pack", "")
	cmArsenal     = pm("Arsenal(26)GunFighter", "Arsenal(26) GunFighter", "")
	cmFancyHand   = pm("FancyHandwork", "Fancy Handwork", "")
	cmModOptions  = pm("ModOptions", "Mod Options", "")
	cmRebalance   = pm("ZRe", "Zombie Rebalance", "")
	cmRV2         = pm("RVInteriors", "RV Interiors", "")
	cmLifestyle   = pm("Lifestyle", "Lifestyle: Hobbies", "")
)

func cpk(id, name, author string, tags []string, about string, dl, likes, days int, mods []CommunityMod, comments ...CommunityComment) CommunityPack {
	return CommunityPack{ID: "p-" + id, Name: name, Author: author, Tags: tags, About: about, Downloads: dl, Likes: likes, Days: days, Mods: mods, Comments: comments}
}

func mods(m ...CommunityMod) []CommunityMod { return m }

var communityPacks = []CommunityPack{
	cpk("vpfull", "Full Viewpoint", "ViewpointVince", tg("Viewpoint", "Heavy"),
		"Every Viewpoint add-on that works on B42 right now. Needs a good PC.", 61200, 5530, 6,
		mods(cmZB, cmViewpoint, cmVPDoor, cmVPLight, cmVPSurface, cmVPRight, cmVPMove, cmVPBlood, cmVPWeather, cmVPFences, cmVPDoors3D, cmVPCurtains, cmVPWindows, cmVPTrees, cmVPStars, cmVPRoofs, cmVPADS, cmVPBallist, cmVPFreeAim, cmVPHeli, cmVPDrive, cmVPSnow, cmVPRagdoll, cmVisceral, cmVoxel),
		cm("Pete_Plays", 2, "runs at 40fps on my 3070. worth it"), cm("NotABot_Dave", 5, "roofs one still WIP, turn it off if you get flicker")),
	cpk("vplite", "Viewpoint Lite", "ViewpointVince", tg("Viewpoint", "Light"),
		"Just the fixes. Third person without the heavy stuff.", 38800, 3100, 12,
		mods(cmZB, cmViewpoint, cmVPDoor, cmVPLight, cmVPSurface, cmVPRight, cmVPMove),
		cm("Kayla_KY", 9, "best starting point if you've never used viewpoint")),
	cpk("vanillaplus", "Vanilla+", "Mortimer", tg("Vanilla+", "Light"),
		"Feels like the base game, just less annoying.", 44100, 3720, 30,
		mods(cmModOptions, cmCommonSense, cmBetterSort, cmSkillJrnl, cmFancyHand, cmHair),
		cm("OldManJenkins", 22, "common sense should be in the base game")),
	cpk("npc", "Not Alone", "rotten_ryan", tg("NPCs", "Viewpoint"),
		"A-Life NPCs with Viewpoint. Survivors roam, trade and shoot at you.", 29400, 2601, 8,
		mods(cmZB, cmViewpoint, cmALife, cmALifeJ, cmALifeVP, cmVPMove, cmVPLight),
		cm("BigRigRandy", 7, "npc stole my car"), cm("rotten_ryan", 7, "working as intended")),
	cpk("guns", "Gun Nut", "Pete_Plays", tg("Guns", "Viewpoint"),
		"Xanny Arms guns with Viewpoint aiming, ballistics and racks to put them on.", 25080, 1890, 15,
		mods(cmZB, cmViewpoint, cmXannyCore, cmXannyGuns, cmXannyVP, cmVPADS, cmVPBallist, cmVPFreeAim, cmVPTactical, cmGunRacks, cmLockers)),
	cpk("looks", "Hair & Faces", "Kayla_KY", tg("Looks", "Light"),
		"More hair, better hair, any colour.", 31900, 2980, 20,
		mods(cmHair, cmSpnHair, cmFluffy, cmYaki, cmVPSize),
		cm("SpiffoFan88", 18, "finally a mullet")),
	cpk("roadtrip", "Life on the Road", "BigRigRandy", tg("Cars", "Viewpoint"),
		"RVs and vans you can live in, plus nicer driving.", 17320, 1240, 25,
		mods(cmZB, cmViewpoint, cmRV, cmVans, cmVPDrive, cmFilibuster, cmAutotsar, cmRV2)),
	cpk("hardcore", "Pain", "NotABot_Dave", tg("Hardcore"),
		"Raiders, helicopters, real music to lure them, and traits that make it worse.", 21780, 1702, 40,
		mods(cmBandits, cmEHE, cmTrueMusic, cmDynTraits, cmMoreTraits, cmRebalance),
		cm("Mortimer", 33, "name checks out"), cm("kentucky_kid", 21, "bandits + helis on day 1 is crazy")),
	cpk("maps", "Bigger Kentucky", "OldManJenkins", tg("Maps"),
		"Two of the big map mods that work on B42.", 15500, 1101, 55,
		mods(cmRavenCreek, cmEerie, cmBetterSort)),
	cpk("weekone", "Before It Started", "kentucky_kid", tg("Story", "Hardcore"),
		"Week One plus Bandits. Start on a normal day and watch it fall apart.", 13300, 1200, 35,
		mods(cmWeekOne, cmBandits, cmTrueMusic, cmAuthZ)),
	cpk("immersion", "Feels Real", "ViewpointVince", tg("Viewpoint", "Heavy"),
		"Weather, snow, ragdolls, gore. Pretty, slow, scary.", 19900, 1670, 10,
		mods(cmZB, cmViewpoint, cmVPWeather, cmVPSnow, cmVPRagdoll, cmVisceral, cmVPBlood, cmVPStars, cmVPHeli)),
	cpk("3ditems", "3D Everything", "SpiffoFan88", tg("Viewpoint", "Heavy"),
		"True 3D items and every 3D model pack.", 12600, 980, 18,
		mods(cmZB, cmViewpoint, cmTrue3D, cmVoxel, cmVPDoors3D, cmVPCurtains, cmVPWindows, cmVPFences, cmGunRacks, cmLockers)),
	cpk("chill", "Sunday Game", "Mortimer", tg("Vanilla+", "Light"),
		"For when friends want to build a base and not die. Hobbies, music, sorting.", 9400, 770, 60,
		mods(cmModOptions, cmCommonSense, cmBetterSort, cmLifestyle, cmTrueMusic, cmSkillJrnl)),
	cpk("melee", "Melee Brutal", "rotten_ryan", tg("Hardcore", "Weapons"),
		"Brita and Arsenal without the guns working half the time. Mostly bats.", 7300, 512, 90,
		mods(cmBrita, cmArsenal, cmFancyHand, cmRebalance)),
	cpk("knoxstarter", "Knox Starter", "Bunker_Brad", tg("Light", "Vanilla+"),
		"What I give friends who just bought the game.", 26100, 2240, 4,
		mods(cmZB, cmModOptions, cmCommonSense, cmHair, cmSkillJrnl, cmSpiffBoy),
		cm("Kayla_KY", 3, "spiffboy is so dumb i love it")),
}
