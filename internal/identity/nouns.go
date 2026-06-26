package identity

// nouns contains 510 unique, title-cased, alphabetic-only nouns for display name generation.
var nouns = [...]string{
	// Animals (mammals) 1-50
	"Capybara", "Platypus", "Llama", "Alpaca", "Wombat",
	"Quokka", "Sloth", "Pangolin", "Armadillo", "Chinchilla",
	"Ferret", "Mongoose", "Gazelle", "Antelope", "Bison",
	"Buffalo", "Caribou", "Reindeer", "Elephant", "Giraffe",
	"Hippo", "Rhino", "Gorilla", "Orangutan", "Chimpanzee",
	"Baboon", "Mandrill", "Tamarin", "Kangaroo", "Wallaby",
	"Opossum", "Chipmunk", "Squirrel", "Porcupine", "Beaver",
	"Muskrat", "Hedgehog", "Badger", "Otter", "Walrus",
	"Manatee", "Dugong", "Narwhal", "Dolphin", "Porpoise",
	"Cheetah", "Leopard", "Jaguar", "Panther", "Mustang",

	// Animals (mammals continued) 51-80
	"Raccoon", "Bobcat", "Jackal", "Dingo", "Corgi",
	"Panda", "Koala", "Moose", "Mammoth", "Yak",
	"Lemur", "Tapir", "Hamster", "Gecko", "Axolotl",
	"Iguana", "Chameleon", "Tortoise", "Alligator", "Crocodile",
	"Salamander", "Newt", "Lobster", "Octopus", "Nautilus",
	"Cuttlefish", "Jellyfish", "Seahorse", "Starfish", "Squid",

	// Birds 81-140
	"Penguin", "Puffin", "Toucan", "Pelican", "Flamingo",
	"Macaw", "Parrot", "Cockatoo", "Lorikeet", "Budgerigar",
	"Falcon", "Kestrel", "Hawk", "Eagle", "Osprey",
	"Condor", "Vulture", "Buzzard", "Harrier", "Merlin",
	"Hummingbird", "Kingfisher", "Albatross", "Gannet", "Cormorant",
	"Crane", "Heron", "Egret", "Ibis", "Stork",
	"Peacock", "Pheasant", "Quail", "Partridge", "Grouse",
	"Ptarmigan", "Pigeon", "Dove", "Cuckoo", "Nightingale",
	"Skylark", "Warbler", "Wren", "Robin", "Swallow",
	"Finch", "Oriole", "Cardinal", "Bluebird", "Sparrow",
	"Starling", "Magpie", "Raven", "Woodpecker", "Sandpiper",
	"Plover", "Avocet", "Lapwing", "Curlew", "Tern",

	// Insects and small creatures 141-170
	"Butterfly", "Dragonfly", "Firefly", "Ladybug", "Mantis",
	"Beetle", "Cricket", "Grasshopper", "Scorpion", "Tarantula",
	"Caterpillar", "Bumblebee", "Honeybee", "Cicada", "Moth",
	"Damselfly", "Wasp", "Hornet", "Centipede", "Millipede",
	"Silkworm", "Glowworm", "Earwig", "Weevil", "Aphid",
	"Mayfly", "Stonefly", "Lacewing", "Leafhopper", "Sawfly",

	// Fish and marine 171-210
	"Salmon", "Trout", "Sturgeon", "Barracuda", "Swordfish",
	"Marlin", "Tuna", "Anchovy", "Herring", "Sardine",
	"Mackerel", "Piranha", "Pufferfish", "Clownfish", "Angelfish",
	"Flounder", "Halibut", "Grouper", "Snapper", "Minnow",
	"Stingray", "Manta", "Hammerhead", "Stickleback", "Guppy",
	"Tetra", "Betta", "Carp", "Perch", "Catfish",
	"Eel", "Lamprey", "Blenny", "Goby", "Wrasse",
	"Triggerfish", "Lionfish", "Boxfish", "Filefish", "Surgeonfish",

	// Shellfish and invertebrates 211-240
	"Scallop", "Oyster", "Mussel", "Clam", "Barnacle",
	"Anemone", "Coral", "Sponge", "Urchin", "Shrimp",
	"Prawn", "Crayfish", "Crawfish", "Krill", "Crab",
	"Hermit", "Conch", "Abalone", "Whelk", "Limpet",
	"Cockle", "Razor", "Nudibranch", "Flatworm", "Tunicate",
	"Bryozoan", "Zooid", "Polyp", "Medusa", "Plankton",

	// Food 241-300
	"Waffle", "Pickle", "Taco", "Biscuit", "Muffin",
	"Pretzel", "Noodle", "Dumpling", "Burrito", "Pancake",
	"Croissant", "Donut", "Gumball", "Nugget", "Popcorn",
	"Cupcake", "Popsicle", "Turnip", "Radish", "Coconut",
	"Avocado", "Kumquat", "Mango", "Papaya", "Kiwi",
	"Lychee", "Truffle", "Brioche", "Focaccia", "Ciabatta",
	"Baguette", "Scone", "Crumpet", "Bagel", "Strudel",
	"Cannoli", "Eclair", "Macaron", "Meringue", "Souffle",
	"Gnocchi", "Ravioli", "Tortellini", "Risotto", "Tempura",
	"Sashimi", "Gyoza", "Wonton", "Samosa", "Falafel",
	"Hummus", "Tahini", "Wasabi", "Kimchi", "Tofu",
	"Edamame", "Quinoa", "Couscous", "Polenta", "Granola",

	// Space and weather 301-340
	"Rocket", "Comet", "Asteroid", "Nebula", "Quasar",
	"Pulsar", "Supernova", "Galaxy", "Cosmos", "Meteor",
	"Eclipse", "Aurora", "Zenith", "Nadir", "Equinox",
	"Solstice", "Perihelion", "Apogee", "Perigee", "Orbit",
	"Tornado", "Blizzard", "Cyclone", "Avalanche", "Volcano",
	"Monsoon", "Typhoon", "Tsunami", "Tempest", "Squall",
	"Gale", "Zephyr", "Chinook", "Sirocco", "Mistral",
	"Breeze", "Gust", "Flurry", "Drizzle", "Deluge",

	// Instruments and music 341-370
	"Banjo", "Kazoo", "Ukulele", "Maraca", "Bongo",
	"Sitar", "Tabla", "Didgeridoo", "Theremin", "Harmonica",
	"Accordion", "Bagpipe", "Bassoon", "Clarinet", "Oboe",
	"Piccolo", "Trombone", "Tuba", "Xylophone", "Glockenspiel",
	"Vibraphone", "Marimba", "Timpani", "Cymbal", "Tambourine",
	"Castanet", "Triangle", "Cowbell", "Dulcimer", "Mandolin",

	// Technology and objects 371-410
	"Gadget", "Widget", "Sprocket", "Trinket", "Pixel",
	"Doodle", "Ratchet", "Cog", "Piston", "Turbine",
	"Dynamo", "Chassis", "Gyroscope", "Pendulum", "Compass",
	"Sextant", "Astrolabe", "Telescope", "Periscope", "Prism",
	"Lantern", "Beacon", "Lighthouse", "Sundial", "Hourglass",
	"Abacus", "Quill", "Inkwell", "Bellows", "Anvil",
	"Crucible", "Mortar", "Pestle", "Cauldron", "Chalice",
	"Goblet", "Flagon", "Tankard", "Decanter", "Amphora",

	// Gems and minerals 411-450
	"Sapphire", "Topaz", "Emerald", "Garnet", "Opal",
	"Jasper", "Onyx", "Agate", "Zircon", "Peridot",
	"Amethyst", "Citrine", "Obsidian", "Granite", "Basalt",
	"Marble", "Slate", "Flint", "Quartz", "Feldspar",
	"Pumice", "Gneiss", "Schist", "Dolomite", "Limestone",
	"Sandstone", "Alabaster", "Turquoise", "Moonstone", "Sunstone",
	"Malachite", "Rhodonite", "Sodalite", "Fluorite", "Calcite",
	"Pyrite", "Hematite", "Magnetite", "Galena", "Cinnabar",

	// Geography and landscape 451-490
	"Geyser", "Canyon", "Glacier", "Meadow", "Prairie",
	"Savanna", "Tundra", "Taiga", "Steppe", "Plateau",
	"Butte", "Mesa", "Fjord", "Lagoon", "Atoll",
	"Peninsula", "Isthmus", "Delta", "Estuary", "Cascade",
	"Ravine", "Gorge", "Gulch", "Hollow", "Thicket",
	"Copse", "Grove", "Canopy", "Bramble", "Thistle",
	"Fern", "Moss", "Lichen", "Mushroom", "Toadstool",
	"Acorn", "Pinecone", "Driftwood", "Pebble", "Boulder",

	// Miscellaneous 491-510
	"Unicorn", "Phoenix", "Griffin", "Dragon", "Kraken",
	"Sphinx", "Minotaur", "Chimera", "Basilisk", "Hydra",
	"Gargoyle", "Golem", "Djinn", "Sprite", "Pixie",
	"Goblin", "Gnome", "Troll", "Ogre", "Banshee",
}
